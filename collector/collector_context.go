package collector

import (
	"mikrotik-exporter/config"

	routeros "github.com/KarolBedkowski/routeros-go-client"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

type collectorContext struct {
	ch        chan<- prometheus.Metric
	device    *config.Device
	client    *routeros.Client
	collector string

	logger log.Logger

	labels []string
}

func newCollectorContext(ch chan<- prometheus.Metric, device *config.Device, client *routeros.Client,
	collector string, logger log.Logger,
) *collectorContext {
	return &collectorContext{
		ch:        ch,
		device:    device,
		client:    client,
		collector: collector,
		labels:    []string{device.Name, device.Address},
		logger:    log.With(logger, "device", device.Name, "collector", collector),
	}
}

func (c *collectorContext) withLabels(labels ...string) *collectorContext {
	return &collectorContext{
		ch:        c.ch,
		device:    c.device,
		client:    c.client,
		collector: c.collector,
		labels:    append([]string{c.device.Name, c.device.Address}, labels...),
		logger:    c.logger,
	}
}
