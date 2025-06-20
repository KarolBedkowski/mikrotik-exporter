package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/hashicorp/go-multierror"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"mikrotik-exporter/collectors"
	"mikrotik-exporter/config"
	"mikrotik-exporter/routeros"
)

var (
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(config.Namespace, "scrape", "collector_duration_seconds"),
		"mikrotik_exporter: duration of a device collector scrape",
		[]string{"device"},
		nil,
	)
	scrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(config.Namespace, "scrape", "collector_success"),
		"mikrotik_exporter: whether a device collector succeeded",
		[]string{"device"},
		nil,
	)
	scrapeErrorsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(config.Namespace, "scrape", "errors"),
		"mikrotik_exporter: number of failed collection per device",
		[]string{"device"},
		nil,
	)
)

// --------------------------------------------

type (
	deviceCollectorRC struct {
		collector collectors.RouterOSCollector
		name      string
	}

	deviceCollector struct {
		logger     *slog.Logger
		cl         *routeros.Client
		device     config.Device
		collectors []deviceCollectorRC
		isSrv      bool
	}
)

func newDeviceCollector(device config.Device, collectors []deviceCollectorRC,
	logger *slog.Logger,
) *deviceCollector {
	if device.TLS {
		if (device.Port) == "" {
			device.Port = config.APIPortTLS
		}
	} else {
		if (device.Port) == "" {
			device.Port = config.APIPort
		}
	}

	if device.Timeout == 0 {
		device.Timeout = config.DefaultTimeout
	}

	return &deviceCollector{
		device:     device,
		collectors: collectors,
		isSrv:      device.Srv != nil,
		logger:     logger.With("device", device.Name),
	}
}

func (dc *deviceCollector) disconnect() {
	// close connection for srv-defined targets
	if dc.isSrv {
		if dc.cl != nil {
			dc.cl.Close()
			dc.cl = nil
		}
	}
}

func (dc *deviceCollector) connect() (*routeros.Client, error) {
	// try do get connection from cache
	if dc.cl != nil {
		// check is connection alive
		if reply, err := dc.cl.Run("/system/identity/print"); err == nil && len(reply.Re) > 0 {
			return dc.cl, nil
		}

		dc.logger.Info("reconnecting")

		// check failed, reconnect
		dc.cl.Close()
		dc.cl = nil
	}

	dc.logger.Debug("trying to Dial")

	conn, err := dc.dial()
	if err != nil {
		return nil, err
	}

	dc.logger.Debug("done dialing")

	client, err := routeros.NewClient(conn)
	if err != nil {
		return nil, fmt.Errorf("create client error: %w", err)
	}

	dc.logger.Debug("got client, trying to login")

	if err := client.Login(dc.device.User, dc.device.Password); err != nil {
		client.Close()

		return nil, fmt.Errorf("login error: %w", err)
	}

	dc.logger.Debug("done with login")
	dc.cl = client

	return client, nil
}

func (dc *deviceCollector) dial() (net.Conn, error) {
	var (
		con     net.Conn
		err     error
		timeout = time.Duration(dc.device.Timeout) * time.Second
	)

	if !dc.device.TLS {
		con, err = net.DialTimeout("tcp", dc.device.Address+":"+dc.device.Port, timeout)
	} else {
		con, err = tls.DialWithDialer(
			&net.Dialer{
				Timeout: timeout,
			},
			"tcp",
			dc.device.Address+":"+dc.device.Port,
			&tls.Config{
				InsecureSkipVerify: dc.device.Insecure, // #nosec
			},
		)
	}

	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}

	return con, nil
}

// collect data for device and return number of failed collectors and
// error if any.
func (dc *deviceCollector) collect(ch chan<- prometheus.Metric) (int, error) {
	client, err := dc.connect()
	if err != nil {
		// no connection so all collectors failed
		return len(dc.collectors), fmt.Errorf("connect error: %w", err)
	}

	defer dc.disconnect()

	var result *multierror.Error

	for _, drc := range dc.collectors {
		logger := dc.logger.With("collector", drc.name)
		ctx := collectors.NewCollectorContext(ch, &dc.device, client, drc.name, logger)

		logger.Debug("start collect")

		if err = drc.collector.Collect(&ctx); err != nil {
			result = multierror.Append(result, fmt.Errorf("collect %s error: %w", drc.name, err))
		}
	}

	if err := result.ErrorOrNil(); err != nil {
		return len(result.Errors), fmt.Errorf("collect error: %w", err)
	}

	return 0, nil
}

func (dc *deviceCollector) getIdentity() error {
	cl, err := dc.connect()
	if err != nil {
		return fmt.Errorf("connect error: %w", err)
	}

	defer dc.disconnect()

	reply, err := cl.Run("/system/identity/print")
	if err != nil {
		return fmt.Errorf("get identity error: %w", err)
	}

	if len(reply.Re) > 0 {
		dc.device.Name = reply.Re[0].Map["name"]
	}

	return nil
}

// --------------------------------------------

type mikrotikCollector struct {
	logger     *slog.Logger
	devices    []*deviceCollector
	collectors []collectors.RouterOSCollector
}

// NewCollector creates a collector instance.
func NewCollector(cfg *config.Config, logger *slog.Logger) prometheus.Collector {
	logger.Info("setting up collector for devices", "numDevices", len(cfg.Devices))

	dcs := make([]*deviceCollector, 0, len(cfg.Devices))
	collectorInstances := createCollectors(cfg, logger)

	for _, dev := range cfg.Devices {
		feat := cfg.DeviceFeatures(dev.Name)
		featNames := feat.FeatureNames()
		dcols := collectorInstances.get(featNames)
		dcs = append(dcs, newDeviceCollector(dev, dcols, logger))

		logger.Debug("new device", "device",
			fmt.Sprintf("%#v", dev), "feat", fmt.Sprintf("%v", featNames))
	}

	colls := collectorInstances.instances()
	c := &mikrotikCollector{
		devices:    dcs,
		collectors: colls,
		logger:     logger,
	}

	return c
}

// Describe implements the prometheus.Collector interface.
func (c *mikrotikCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
	ch <- scrapeErrorsDesc

	for _, co := range c.collectors {
		co.Describe(ch)
	}
}

// Collect implements the prometheus.Collector interface.
func (c *mikrotikCollector) Collect(ch chan<- prometheus.Metric) {
	_, _ = daemon.SdNotify(false, "STATUS=collecting")

	wg := sync.WaitGroup{}
	realDevices := make([]*deviceCollector, 0, len(c.devices))

	for _, dc := range c.devices {
		if dc.isSrv {
			if devs, err := c.devicesFromSrv(dc); err == nil {
				realDevices = append(realDevices, devs...)
			} else {
				c.logger.Error("resolve srv error", "err", err)
			}
		} else {
			realDevices = append(realDevices, dc)
		}
	}

	wg.Add(len(realDevices))

	for _, dev := range realDevices {
		go func(d *deviceCollector) {
			c.collectFromDevice(d, ch)
			wg.Done()
		}(dev)
	}

	wg.Wait()

	_, _ = daemon.SdNotify(false, "STATUS=waiting")
}

func (c *mikrotikCollector) collectFromDevice(d *deviceCollector, ch chan<- prometheus.Metric) {
	name := d.device.Name
	logger := c.logger.With("device", name)
	logger.Debug("start collect for device")

	begin := time.Now()
	numFailed, err := d.collect(ch)
	duration := time.Since(begin)
	success := 0.0

	if err != nil {
		logger.Error(fmt.Sprintf("collector failed after %fs", duration.Seconds()), "err", err)
	} else {
		logger.Debug(fmt.Sprintf("collector succeeded after %fs", duration.Seconds()))

		success = 1
	}

	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue,
		duration.Seconds(), name)
	ch <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, success, name)
	ch <- prometheus.MustNewConstMetric(scrapeErrorsDesc, prometheus.GaugeValue,
		float64(numFailed), name)
}

// --------------------------------------------

func (c *mikrotikCollector) devicesFromSrv(devCol *deviceCollector) ([]*deviceCollector, error) {
	dev := devCol.device

	r, err := resolveServices(dev.Srv.DNS, dev.Srv.Record)
	if err != nil {
		return nil, fmt.Errorf("dns query for %s error: %w", dev.Srv.Record, err)
	}

	realDevices := make([]*deviceCollector, 0, len(r))

	for _, target := range r {
		d := config.Device{
			Name:     target,
			Address:  target,
			User:     dev.User,
			Password: dev.Password,
			Srv:      dev.Srv,
		}

		ndc := newDeviceCollector(d, devCol.collectors, devCol.logger)
		if err := ndc.getIdentity(); err != nil {
			c.logger.Error("error fetching identity",
				"device", devCol.device.Name, "error", err)

			continue
		}

		realDevices = append(realDevices, ndc)
	}

	return realDevices, nil
}

// --------------------------------------------

var ErrNoServersDefined = errors.New("no servers defined")

func resolveServices(srvDNS *config.DNSServer, record string) ([]string, error) {
	var dnsServer string

	if srvDNS == nil {
		conf, err := dns.ClientConfigFromFile("/etc/resolv.conf")
		if err != nil {
			return nil, fmt.Errorf("load resolv.conf file error: %w", err)
		}

		if conf == nil || len(conf.Servers) == 0 {
			return nil, ErrNoServersDefined
		}

		dnsServer = net.JoinHostPort(conf.Servers[0], strconv.Itoa(config.DNSPort))
	} else {
		dnsServer = net.JoinHostPort(srvDNS.Address, strconv.Itoa(srvDNS.Port))
	}

	slog.Debug("resolve services", "dns_server", dnsServer, "record", record)

	dnsMsg := new(dns.Msg)
	dnsMsg.RecursionDesired = true
	dnsMsg.SetQuestion(dns.Fqdn(record), dns.TypeSRV)

	dnsCli := new(dns.Client)

	r, _, err := dnsCli.Exchange(dnsMsg, dnsServer)
	if err != nil {
		return nil, fmt.Errorf("dns query for %s error: %w", record, err)
	}

	result := make([]string, 0, len(r.Answer))

	for _, k := range r.Answer {
		if s, ok := k.(*dns.SRV); ok {
			slog.Debug("resolved services",
				"dns_server", dnsServer, "record", record, "result", s.Target)

			result = append(result, strings.TrimRight(s.Target, "."))
		}
	}

	return result, nil
}

// --------------------------------------------

type collectorInstances map[string]collectors.RouterOSCollector

// createCollectors create instances of collectors according to configuration.
func createCollectors(cfg *config.Config, logger *slog.Logger) collectorInstances {
	colls := make(map[string]collectors.RouterOSCollector)

	for _, k := range cfg.AllEnabledFeatures() {
		colls[k] = collectors.InstanateCollector(k)
		logger.Debug("new collector", "collector", k)
	}

	return colls
}

func (ci collectorInstances) get(names []string) []deviceCollectorRC {
	dcols := make([]deviceCollectorRC, 0, len(names))

	for _, n := range names {
		dcols = append(dcols, deviceCollectorRC{ci[n], n})
	}

	return dcols
}

func (ci collectorInstances) instances() []collectors.RouterOSCollector {
	colls := make([]collectors.RouterOSCollector, 0, len(ci))

	for _, c := range ci {
		colls = append(colls, c)
	}

	return colls
}
