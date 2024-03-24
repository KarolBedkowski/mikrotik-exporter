package main

import (
	// #nosec
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

	"mikrotik-exporter/collectors"
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

type (
	deviceCollectorRC struct {
		name      string
		collector collectors.RouterOSCollector
	}

	deviceCollector struct {
		device     config.Device
		collectors []deviceCollectorRC
		cl         *routeros.Client
		isSrv      bool

		logger   log.Logger
		connLock sync.Mutex
	}
)

func newDeviceCollector(device config.Device, collectors []deviceCollectorRC,
	logger log.Logger,
) *deviceCollector {
	if device.TLS {
		if (device.Port) == "" {
			device.Port = apiPortTLS
		}
	} else {
		if (device.Port) == "" {
			device.Port = apiPort
		}
	}

	if device.Timeout == 0 {
		device.Timeout = DefaultTimeout
	}

	return &deviceCollector{
		device:     device,
		collectors: collectors,
		isSrv:      device.Srv != nil,
		logger:     log.WithSuffix(logger, "device", device.Name),
	}
}

func (dc *deviceCollector) close() {
	dc.connLock.Lock()
	defer dc.connLock.Unlock()

	// close connection for srv-defined targets
	if dc.isSrv {
		if dc.cl != nil {
			dc.cl.Close()
			dc.cl = nil
		}
	}
}

func (dc *deviceCollector) getConnection() (*routeros.Client, error) {
	dc.connLock.Lock()
	defer dc.connLock.Unlock()

	// try do get connection from cache
	if dc.cl != nil {
		// check is connection alive
		if reply, err := dc.cl.Run("/system/identity/print"); err == nil && len(reply.Re) > 0 {
			return dc.cl, nil
		}

		// check failed, reconnect
		dc.cl.Close()
		dc.cl = nil

		_ = level.Info(dc.logger).Log("msg", "reconnecting")
	}

	client, err := dc.connect()
	if err == nil {
		dc.cl = client
	}

	return client, err
}

func (dc *deviceCollector) connect() (*routeros.Client, error) {
	_ = level.Debug(dc.logger).Log("msg", "trying to Dial")

	conn, err := dc.dial()
	if err != nil {
		return nil, err
	}

	_ = level.Debug(dc.logger).Log("msg", "done dialing")

	client, err := routeros.NewClient(conn)
	if err != nil {
		return nil, fmt.Errorf("create client error: %w", err)
	}

	_ = level.Debug(dc.logger).Log("msg", "got client, trying to login")

	if err := dc.login(client); err != nil {
		client.Close()

		return nil, err
	}

	_ = level.Debug(dc.logger).Log("msg", "done wth login")

	return client, nil
}

func (dc *deviceCollector) dial() (net.Conn, error) {
	timeout := time.Duration(dc.device.Timeout) * time.Second

	if !dc.device.TLS {
		con, err := net.DialTimeout("tcp", dc.device.Address+":"+dc.device.Port, timeout)
		if err != nil {
			return nil, fmt.Errorf("dial error: %w", err)
		}

		return con, nil
	}

	con, err := tls.DialWithDialer(
		&net.Dialer{
			Timeout: timeout,
		},
		"tcp",
		dc.device.Address+":"+dc.device.Port,
		&tls.Config{
			InsecureSkipVerify: dc.device.Insecure, // #nosec
		},
	)
	if err != nil {
		return nil, fmt.Errorf("dial with dialler error: %w", err)
	}

	return con, nil
}

var ErrLoginNoRet = errors.New("RouterOS: /login: no ret (challenge) received")

func (dc *deviceCollector) login(client *routeros.Client) error {
	r, err := client.Run("/login", "=name="+dc.device.User, "=password="+dc.device.Password)
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

	if _, err = client.Run("/login", "=name="+dc.device.User,
		"=response="+challengeResponse(b, dc.device.Password)); err != nil {
		return fmt.Errorf("logins send response error: %w", err)
	}

	return nil
}

// connectAndCollect collect data for device and return number of failed collectors and
// error if any.
func (dc *deviceCollector) connectAndCollect(ch chan<- prometheus.Metric) (int, error) {
	client, err := dc.getConnection()
	if err != nil {
		// all collectors failed
		return len(dc.collectors), fmt.Errorf("connect error: %w", err)
	}

	defer dc.close()

	var (
		result    *multierror.Error
		numFailed int
	)

	for _, drc := range dc.collectors {
		logger := log.WithSuffix(dc.logger, "collector", drc.name)
		ctx := collectors.NewCollectorContext(ch, &dc.device, client, drc.name, logger)

		_ = level.Debug(logger).Log("msg", "start collect")

		if err = drc.collector.Collect(ctx); err != nil {
			result = multierror.Append(result, fmt.Errorf("collect %s error: %w", drc.name, err))
			numFailed++
		}
	}

	if err := result.ErrorOrNil(); err != nil {
		return numFailed, fmt.Errorf("collect error: %w", err)
	}

	return 0, nil
}

func (dc *deviceCollector) getIdentity() error {
	cl, err := dc.getConnection()
	if err != nil {
		return fmt.Errorf("connect error: %w", err)
	}

	defer dc.close()

	reply, err := cl.Run("/system/identity/print")
	if err != nil {
		return fmt.Errorf("get identity error: %w", err)
	}

	if len(reply.Re) > 0 {
		dc.device.Name = reply.Re[0].Map["name"]
	}

	return nil
}

type collector struct {
	devices    []*deviceCollector
	collectors map[string]collectors.RouterOSCollector
	logger     log.Logger
}

// NewCollector creates a collector instance.
func NewCollector(cfg *config.Config, logger log.Logger) prometheus.Collector {
	_ = level.Info(logger).Log("msg", "setting up collector for devices", "numDevices",
		len(cfg.Devices))

	dcs := make([]*deviceCollector, 0, len(cfg.Devices))

	collectors := createCollectors(cfg, logger)

	for _, dev := range cfg.Devices {
		feat, err := cfg.DeviceFeatures(dev.Name)
		if err != nil {
			panic(err)
		}

		var dcols []deviceCollectorRC

		featNames := feat.FeatureNames()
		for _, n := range featNames {
			dcols = append(dcols, deviceCollectorRC{n, collectors[n]})
		}

		dc := newDeviceCollector(dev, dcols, logger)
		dcs = append(dcs, dc)

		_ = level.Debug(logger).Log("msg", "new device", "device",
			fmt.Sprintf("%#v", &dc.device), "feat", fmt.Sprintf("%v", featNames))
	}

	c := &collector{
		devices:    dcs,
		collectors: createCollectors(cfg, logger),
		logger:     logger,
	}

	return c
}

// Describe implements the prometheus.Collector interface.
func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
	ch <- scrapeErrorsDesc

	for _, co := range c.collectors {
		co.Describe(ch)
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

			ndc := newDeviceCollector(d, devCol.collectors, devCol.logger)
			if err := ndc.getIdentity(); err != nil {
				_ = level.Error(c.logger).Log("msg", "error fetching identity",
					"device", devCol.device.Name, "error", err)
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
			_ = level.Info(c.logger).Log("msg", "SRV configuration detected",
				"SRV", dc.device.Srv.Record)

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

func (c *collector) collectForDevice(d *deviceCollector, ch chan<- prometheus.Metric) {
	name := d.device.Name
	logger := log.WithSuffix(c.logger, "device", name)
	_ = level.Debug(logger).Log("msg", "start collect for device")

	begin := time.Now()
	numFailed, err := d.connectAndCollect(ch)
	duration := time.Since(begin)

	success := 0.0

	if err != nil {
		_ = level.Error(logger).Log("msg", fmt.Sprintf("ERROR: collector failed after %fs",
			duration.Seconds()), "err", err)
	} else {
		_ = level.Debug(logger).Log("msg", fmt.Sprintf("OK: collector succeeded after %fs",
			duration.Seconds()))

		success = 1
	}

	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds(), name)
	ch <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, success, name)
	ch <- prometheus.MustNewConstMetric(scrapeErrorsDesc, prometheus.GaugeValue, float64(numFailed), name)
}

func challengeResponse(cha []byte, password string) string {
	h := md5.New() // #nosec
	h.Write([]byte{0})
	_, _ = io.WriteString(h, password)
	h.Write(cha)

	return fmt.Sprintf("00%x", h.Sum(nil))
}

func createCollectors(cfg *config.Config, logger log.Logger) map[string]collectors.RouterOSCollector {
	colls := make(map[string]collectors.RouterOSCollector)
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
		colls[k] = collectors.InstanateCollector(k)
		_ = level.Debug(logger).Log("msg", "new collector", "collector", k)
	}

	return colls
}
