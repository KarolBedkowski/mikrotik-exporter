package collector

//
// mikrotik_collector.go
//
// Distributed under terms of the GPLv3 license.
//

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"mikrotik-exporter/internal/collectors"
	"mikrotik-exporter/internal/config"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/prometheus/client_golang/prometheus"
)

// --------------------------------------------

var (
	scrapeDeviceDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(config.Namespace, "scrape", "device_duration_seconds"),
		"mikrotik_exporter: duration of a device collector scrape",
		[]string{"dev_name", "dev_address"},
		nil,
	)
	scrapeDeviceSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(config.Namespace, "scrape", "device_success"),
		"mikrotik_exporter: whether a device collector succeeded",
		[]string{"dev_name", "dev_address"},
		nil,
	)
)

// --------------------------------------------

type mikrotikCollector struct {
	logger     *slog.Logger
	devices    []*deviceCollector
	collectors []collectors.RouterOSCollector
}

// NewCollector creates a collector instance.
func NewCollector(cfg *config.Config) prometheus.Collector {
	logger := slog.Default()
	logger.Info("setting up collector for devices", "numDevices", len(cfg.Devices))

	dcs := make([]*deviceCollector, 0, len(cfg.Devices))
	collectorInstances := createCollectors(cfg)

	for _, dev := range cfg.Devices {
		feat := cfg.DeviceFeatures(dev.Name)
		featNames := feat.FeatureNames()
		dcols := collectorInstances.get(featNames, feat)
		dcs = append(dcs, newDeviceCollector(dev, dcols))

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
	ch <- scrapeDeviceDurationDesc
	ch <- scrapeDeviceSuccessDesc
	ch <- scrapeCollectorErrorsDesc

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
	address, name := d.device.Address, d.device.Name

	logger := c.logger.With("device", name)
	logger.Debug("start collect for device", "device", &d.device)

	begin := time.Now()
	err := d.collect(ch)
	duration := time.Since(begin)

	if err != nil {
		logger.Error(fmt.Sprintf("collector failed after %fs", duration.Seconds()), "err", err)
		ch <- prometheus.MustNewConstMetric(scrapeDeviceSuccessDesc, prometheus.GaugeValue, 0.0, name, address)
	} else {
		logger.Debug(fmt.Sprintf("collector succeeded after %fs", duration.Seconds()))
		ch <- prometheus.MustNewConstMetric(scrapeDeviceSuccessDesc, prometheus.GaugeValue, 1.0, name, address)
	}

	ch <- prometheus.MustNewConstMetric(scrapeDeviceDurationDesc, prometheus.GaugeValue, duration.Seconds(),
		name, address)
}

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

		realDevices = append(realDevices, newDeviceCollector(d, devCol.collectors))
	}

	return realDevices, nil
}
