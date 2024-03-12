package collector

import (
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"mikrotik-exporter/config"

	"github.com/KarolBedkowski/routeros-go-client"
	"github.com/hashicorp/go-multierror"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const (
	namespace  = "mikrotik"
	apiPort    = "8728"
	apiPortTLS = "8729"
	dnsPort    = 53

	// DefaultTimeout defines the default timeout when connecting to a router.
	DefaultTimeout = 5 * time.Second
)

var (
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_duration_seconds"),
		"mikrotik_exporter: duration of a device collector scrape",
		[]string{"device"},
		nil,
	)
	scrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_success"),
		"mikrotik_exporter: whether a device collector succeeded",
		[]string{"device"},
		nil,
	)
)

type deviceCollector struct {
	device     config.Device
	collectors []string
	cl         *routeros.Client
	isSrv      bool
}

type collector struct {
	devices     []*deviceCollector
	collectors  map[string]routerOSCollector
	timeout     time.Duration
	enableTLS   bool
	insecureTLS bool
	connLock    sync.Mutex
}

// WithTimeout sets timeout for connecting to router.
func WithTimeout(d time.Duration) Option {
	return func(c *collector) {
		c.timeout = d
	}
}

// WithTLS enables TLS.
func WithTLS(insecure bool) Option {
	return func(c *collector) {
		c.enableTLS = true
		c.insecureTLS = insecure
	}
}

// Option applies options to collector.
type Option func(*collector)

// NewCollector creates a collector instance.
func NewCollector(cfg *config.Config, opts ...Option) (prometheus.Collector, error) {
	log.WithFields(log.Fields{
		"numDevices": len(cfg.Devices),
	}).Info("setting up collector for devices")

	dcs := make([]*deviceCollector, 0, len(cfg.Devices))

	for _, d := range cfg.Devices {
		feat, err := cfg.DeviceFeatures(d.Name)
		if err != nil {
			panic(err)
		}

		featNames := feat.FeatureNames()
		dc := &deviceCollector{d, featNames, nil, (config.SrvRecord{}) != d.Srv}
		dcs = append(dcs, dc)

		log.WithFields(log.Fields{"device": &dc}).Debug("new device")
	}

	c := &collector{
		devices:    dcs,
		collectors: newCollectors(cfg),
		timeout:    DefaultTimeout,
	}

	for _, o := range opts {
		o(c)
	}

	return c, nil
}

// Describe implements the prometheus.Collector interface.
func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc

	for _, co := range c.collectors {
		co.describe(ch)
	}
}

func (c *collector) srvToDevice(devCol *deviceCollector) []*deviceCollector {
	dev := devCol.device
	conf, _ := dns.ClientConfigFromFile("/etc/resolv.conf")
	dnsServer := net.JoinHostPort(conf.Servers[0], strconv.Itoa(dnsPort))

	if (config.DnsServer{}) != dev.Srv.Dns {
		dnsServer = net.JoinHostPort(dev.Srv.Dns.Address, strconv.Itoa(dev.Srv.Dns.Port))
		log.WithFields(log.Fields{"DnsServer": dnsServer}).Info("Custom DNS config detected")
	}

	dnsMsg := new(dns.Msg)
	dnsMsg.RecursionDesired = true
	dnsMsg.SetQuestion(dns.Fqdn(dev.Srv.Record), dns.TypeSRV)

	dnsCli := new(dns.Client)

	r, _, err := dnsCli.Exchange(dnsMsg, dnsServer)
	if err != nil {
		panic(err)
	}

	var realDevices []*deviceCollector

	for _, k := range r.Answer {
		if s, ok := k.(*dns.SRV); ok {
			d := config.Device{
				Name:     strings.TrimRight(s.Target, "."),
				Address:  strings.TrimRight(s.Target, "."),
				User:     dev.User,
				Password: dev.Password,
			}

			ndc := &deviceCollector{d, devCol.collectors, nil, true}
			_ = c.getIdentity(ndc)
			realDevices = append(realDevices, ndc)
		}
	}

	return realDevices
}

// Collect implements the prometheus.Collector interface.
func (c *collector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}

	var realDevices []*deviceCollector

	for _, dc := range c.devices {
		if dc.isSrv {
			log.WithFields(log.Fields{"SRV": dc.device.Srv.Record}).Info("SRV configuration detected")
			realDevices = append(realDevices, c.srvToDevice(dc)...)
		} else {
			realDevices = append(realDevices, dc)
		}
	}

	wg.Add(len(realDevices))

	for _, dev := range realDevices {
		go func(d *deviceCollector) {
			c.collectForDevice(d, ch)
			wg.Done()
		}(dev)
	}

	wg.Wait()
}

func (c *collector) getIdentity(devCol *deviceCollector) error {
	cl, err := c.getConnection(devCol)
	if err != nil {
		log.WithFields(log.Fields{
			"device": devCol.device.Name,
			"error":  err,
		}).Error("error dialing device fetching identity")

		return err
	}

	defer c.closeConnection(devCol)

	reply, err := cl.Run("/system/identity/print")
	if err != nil {
		log.WithFields(log.Fields{
			"device": devCol.device.Name,
			"error":  err,
		}).Error("error fetching identity")

		return fmt.Errorf("get identity error: %w", err)
	}

	if len(reply.Re) > 0 {
		devCol.device.Name = reply.Re[0].Map["name"]
	}

	return nil
}

func (c *collector) collectForDevice(d *deviceCollector, ch chan<- prometheus.Metric) {
	name := d.device.Name
	log.WithFields(log.Fields{"device": name}).Debug("start collect for device")

	begin := time.Now()
	err := c.connectAndCollect(d, ch)
	duration := time.Since(begin)

	success := 0.0

	if err != nil {
		log.Errorf("ERROR: %s collector failed after %fs: %s", name, duration.Seconds(), err)
	} else {
		log.Debugf("OK: %s collector succeeded after %fs.", name, duration.Seconds())

		success = 1
	}

	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds(), name)
	ch <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, success, name)
}

func (c *collector) connectAndCollect(d *deviceCollector, ch chan<- prometheus.Metric) error {
	client, err := c.getConnection(d)
	if err != nil {
		log.WithFields(log.Fields{
			"device": d.device.Name,
			"error":  err,
		}).Error("error dialing device")

		return err
	}

	defer c.closeConnection(d)

	var result *multierror.Error

	for _, coName := range d.collectors {
		co := c.collectors[coName]
		ctx := &collectorContext{ch, &d.device, client}
		log.WithFields(log.Fields{
			"device":    d.device.Name,
			"collector": fmt.Sprintf("%#v", co),
		}).Debug("collect")

		if err = co.collect(ctx); err != nil {
			result = multierror.Append(result, fmt.Errorf("collecting by %s error: %w", coName, err))
		}
	}

	return result.ErrorOrNil()
}

func (c *collector) getConnection(devCol *deviceCollector) (*routeros.Client, error) {
	c.connLock.Lock()
	defer c.connLock.Unlock()

	// try do get connection from cache
	if devCol.cl != nil {
		// check is connection alive
		if reply, err := devCol.cl.Run("/system/identity/print"); err == nil && len(reply.Re) > 0 {
			return devCol.cl, nil
		}

		// check failed, reconnect
		devCol.cl.Close()
		devCol.cl = nil

		log.WithFields(log.Fields{"device": devCol.device.Name}).Info("reconnecting")
	}

	client, err := c.connect(&devCol.device)
	if err == nil {
		devCol.cl = client
	}

	return client, err
}

func (c *collector) closeConnection(dc *deviceCollector) {
	c.connLock.Lock()
	defer c.connLock.Unlock()

	// close connection for srv-defined targets
	if dc.isSrv {
		if dc.cl != nil {
			dc.cl.Close()
			dc.cl = nil
		}
	}
}

func (c *collector) dial(d *config.Device) (net.Conn, error) {
	if !c.enableTLS {
		if (d.Port) == "" {
			d.Port = apiPort
		}

		return net.DialTimeout("tcp", d.Address+":"+d.Port, c.timeout)
	}

	tlsCfg := &tls.Config{
		InsecureSkipVerify: c.insecureTLS,
	}

	if (d.Port) == "" {
		d.Port = apiPortTLS
	}

	return tls.DialWithDialer(&net.Dialer{Timeout: c.timeout},
		"tcp", d.Address+":"+d.Port, tlsCfg)
}

func (c *collector) login(d *config.Device, client *routeros.Client) error {
	r, err := client.Run("/login", "=name="+d.User, "=password="+d.Password)
	if err != nil {
		return fmt.Errorf("run login error: %w", err)
	}

	ret, ok := r.Done.Map["ret"]
	if !ok {
		// Login method post-6.43 one stage, cleartext and no challenge
		if r.Done != nil {
			return nil
		}

		return errors.New("RouterOS: /login: no ret (challenge) received")
	}

	// Login method pre-6.43 two stages, challenge
	b, err := hex.DecodeString(ret)
	if err != nil {
		return fmt.Errorf("RouterOS: /login: invalid ret (challenge) hex string received: %w", err)
	}

	if _, err = client.Run("/login", "=name="+d.User, "=response="+challengeResponse(b, d.Password)); err != nil {
		return fmt.Errorf("logins send response error: %w", err)
	}

	return nil
}

func (c *collector) connect(dev *config.Device) (*routeros.Client, error) {
	log.WithField("device", dev.Name).Debug("trying to Dial")

	conn, err := c.dial(dev)
	if err != nil {
		return nil, err
	}

	log.WithField("device", dev.Name).Debug("done dialing")

	client, err := routeros.NewClient(conn)
	if err != nil {
		return nil, fmt.Errorf("create client error: %w", err)
	}

	log.WithField("device", dev.Name).Debug("got client, trying to login")

	if err := c.login(dev, client); err != nil {
		client.Close()

		return nil, err
	}

	log.WithField("device", dev.Name).Debug("done wth login")

	return client, nil
}

func challengeResponse(cha []byte, password string) string {
	h := md5.New()
	h.Write([]byte{0})
	_, _ = io.WriteString(h, password)
	h.Write(cha)

	return fmt.Sprintf("00%x", h.Sum(nil))
}

func newCollectors(cfg *config.Config) map[string]routerOSCollector {
	collectors := make(map[string]routerOSCollector)

	uniqueNames := make(map[string]struct{})
	for _, name := range cfg.Features.FeatureNames() {
		uniqueNames[name] = struct{}{}
	}

	for _, dev := range cfg.Devices {
		if len(dev.Profile) > 0 {
			features, err := cfg.DeviceFeatures(dev.Name)
			if err != nil {
				panic(err)
			}

			for _, name := range features.FeatureNames() {
				uniqueNames[name] = struct{}{}
			}
		}
	}

	for k := range uniqueNames {
		collectors[k] = instanateCollector(k)
		log.WithFields(log.Fields{"collector": k}).Debug("new collector")
	}

	return collectors
}
