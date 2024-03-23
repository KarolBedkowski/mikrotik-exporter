package collector

import (
	"crypto/md5" // #nosec
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
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/hashicorp/go-multierror"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace  = "mikrotik"
	apiPort    = "8728"
	apiPortTLS = "8729"
	dnsPort    = 53

	// DefaultTimeout defines the default timeout when connecting to a router.
	DefaultTimeout = 5
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
	scrapeErrorsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "errors"),
		"mikrotik_exporter: number of failed collection per device",
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
	devices    []*deviceCollector
	collectors map[string]routerOSCollector
	logger     log.Logger
	connLock   sync.Mutex
}

// NewCollector creates a collector instance.
func NewCollector(cfg *config.Config, logger log.Logger) (prometheus.Collector, error) {
	_ = level.Info(logger).Log("msg", "setting up collector for devices", "numDevices", len(cfg.Devices))

	dcs := make([]*deviceCollector, 0, len(cfg.Devices))

	for _, d := range cfg.Devices {
		feat, err := cfg.DeviceFeatures(d.Name)
		if err != nil {
			panic(err)
		}

		featNames := feat.FeatureNames()
		dc := &deviceCollector{d, featNames, nil, d.Srv != nil}
		dcs = append(dcs, dc)

		_ = level.Debug(logger).Log("msg", "new device", "device",
			fmt.Sprintf("%#v", &dc.device), "feat", fmt.Sprintf("%v", featNames))
	}

	c := &collector{
		devices:    dcs,
		collectors: newCollectors(cfg, logger),
		logger:     logger,
	}

	return c, nil
}

// Describe implements the prometheus.Collector interface.
func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
	ch <- scrapeErrorsDesc

	for _, co := range c.collectors {
		co.describe(ch)
	}
}

func (c *collector) srvToDevice(devCol *deviceCollector) []*deviceCollector {
	dev := devCol.device
	conf, _ := dns.ClientConfigFromFile("/etc/resolv.conf")
	dnsServer := net.JoinHostPort(conf.Servers[0], strconv.Itoa(dnsPort))

	if dev.Srv.DNS != nil {
		dnsServer = net.JoinHostPort(dev.Srv.DNS.Address, strconv.Itoa(dev.Srv.DNS.Port))
		_ = level.Info(c.logger).Log("msg", "Custom DNS config detected", "DNSServer", dnsServer)
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

			if err := c.getIdentity(ndc); err != nil {
				_ = level.Error(c.logger).Log("msg", "error fetching identity", "device", devCol.device.Name, "error", err)
			}

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
			_ = level.Info(c.logger).Log("msg", "SRV configuration detected", "SRV", dc.device.Srv.Record)
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
		return fmt.Errorf("get connection error: %w", err)
	}

	defer c.closeConnection(devCol)

	reply, err := cl.Run("/system/identity/print")
	if err != nil {
		return fmt.Errorf("get identity error: %w", err)
	}

	if len(reply.Re) > 0 {
		devCol.device.Name = reply.Re[0].Map["name"]
	}

	return nil
}

func (c *collector) collectForDevice(d *deviceCollector, ch chan<- prometheus.Metric) {
	name := d.device.Name
	logger := log.With(c.logger, "device", name)
	_ = level.Debug(logger).Log("msg", "start collect for device")

	begin := time.Now()
	err := c.connectAndCollect(d, ch, logger)
	duration := time.Since(begin)

	success := 0.0

	if err != nil {
		_ = level.Error(logger).Log("msg", fmt.Sprintf("ERROR: collector failed after %fs", duration.Seconds()), "err", err)
	} else {
		_ = level.Debug(logger).Log("msg", fmt.Sprintf("OK: collector succeeded after %fs", duration.Seconds()))

		success = 1
	}

	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds(), name)
	ch <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, success, name)
}

func (c *collector) connectAndCollect(devCollector *deviceCollector, ch chan<- prometheus.Metric,
	logger log.Logger,
) error {
	client, err := c.getConnection(devCollector)
	if err != nil {
		return fmt.Errorf("connect error: %w", err)
	}

	defer c.closeConnection(devCollector)

	var (
		result    *multierror.Error
		numFailed int
	)

	for _, coName := range devCollector.collectors {
		co := c.collectors[coName]
		ctx := newCollectorContext(ch, &devCollector.device, client, coName, logger)
		_ = level.Debug(logger).Log("msg", "collect", "device", devCollector.device.Name, "collector", coName)

		if err = co.collect(ctx); err != nil {
			result = multierror.Append(result, fmt.Errorf("collecting by %s error: %w", coName, err))
			numFailed++
		}
	}

	ch <- prometheus.MustNewConstMetric(scrapeErrorsDesc, prometheus.GaugeValue,
		float64(numFailed), devCollector.device.Name)

	if err := result.ErrorOrNil(); err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	return nil
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

		_ = level.Info(c.logger).Log("msg", "reconnecting", "device", devCol.device.Name)
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

func (c *collector) dial(device *config.Device) (net.Conn, error) {
	timeout := time.Duration(DefaultTimeout) * time.Second
	if device.Timeout > 0 {
		timeout = time.Duration(device.Timeout) * time.Second
	}

	if !device.TLS {
		if (device.Port) == "" {
			device.Port = apiPort
		}

		con, err := net.DialTimeout("tcp", device.Address+":"+device.Port, timeout)
		if err != nil {
			return nil, fmt.Errorf("dial error: %w", err)
		}

		return con, nil
	}

	tlsCfg := &tls.Config{
		InsecureSkipVerify: device.Insecure, // #nosec
	}

	if (device.Port) == "" {
		device.Port = apiPortTLS
	}

	con, err := tls.DialWithDialer(
		&net.Dialer{Timeout: timeout},
		"tcp", device.Address+":"+device.Port, tlsCfg)
	if err != nil {
		return nil, fmt.Errorf("dial with dialler error: %w", err)
	}

	return con, nil
}

var ErrLoginNoRet = errors.New("RouterOS: /login: no ret (challenge) received")

func (c *collector) login(device *config.Device, client *routeros.Client) error {
	r, err := client.Run("/login", "=name="+device.User, "=password="+device.Password)
	if err != nil {
		return fmt.Errorf("run login error: %w", err)
	}

	ret, ok := r.Done.Map["ret"]
	if !ok {
		// Login method post-6.43 one stage, cleartext and no challenge
		if r.Done != nil {
			return nil
		}

		return ErrLoginNoRet
	}

	// Login method pre-6.43 two stages, challenge
	b, err := hex.DecodeString(ret)
	if err != nil {
		return fmt.Errorf("RouterOS: /login: invalid ret (challenge) hex string received: %w", err)
	}

	if _, err = client.Run("/login", "=name="+device.User,
		"=response="+challengeResponse(b, device.Password)); err != nil {
		return fmt.Errorf("logins send response error: %w", err)
	}

	return nil
}

func (c *collector) connect(dev *config.Device) (*routeros.Client, error) {
	logger := log.With(c.logger, "device", dev.Name)
	_ = level.Debug(logger).Log("msg", "trying to Dial")

	conn, err := c.dial(dev)
	if err != nil {
		return nil, err
	}

	_ = level.Debug(logger).Log("msg", "done dialing")

	client, err := routeros.NewClient(conn)
	if err != nil {
		return nil, fmt.Errorf("create client error: %w", err)
	}

	_ = level.Debug(logger).Log("msg", "got client, trying to login")

	if err := c.login(dev, client); err != nil {
		client.Close()

		return nil, err
	}

	_ = level.Debug(logger).Log("msg", "done wth login")

	return client, nil
}

func challengeResponse(cha []byte, password string) string {
	h := md5.New() // #nosec
	h.Write([]byte{0})
	_, _ = io.WriteString(h, password)
	h.Write(cha)

	return fmt.Sprintf("00%x", h.Sum(nil))
}

func newCollectors(cfg *config.Config, logger log.Logger) map[string]routerOSCollector {
	collectors := make(map[string]routerOSCollector)

	uniqueNames := make(map[string]struct{})

	applyDefault := false

	for _, dev := range cfg.Devices {
		if dev.Profile == "" {
			applyDefault = true
		} else {
			features, err := cfg.DeviceFeatures(dev.Name)
			if err != nil {
				panic(err)
			}

			for _, name := range features.FeatureNames() {
				uniqueNames[name] = struct{}{}
			}
		}
	}

	if applyDefault {
		for _, name := range cfg.Features.FeatureNames() {
			uniqueNames[name] = struct{}{}
		}
	}

	for k := range uniqueNames {
		collectors[k] = instanateCollector(k)
		level.Debug(logger).Log("msg", "new collector", "collector", k)
	}

	return collectors
}
