package collector

import (
	"mikrotik-exporter/config"

	routeros "github.com/KarolBedkowski/routeros-go-client"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type collectorContext struct {
	ch        chan<- prometheus.Metric
	device    *config.Device
	client    *routeros.Client
	collector string

	labels []string
}

func newCollectorContext(ch chan<- prometheus.Metric, device *config.Device, client *routeros.Client,
	collector string,
) *collectorContext {
	return &collectorContext{
		ch:        ch,
		device:    device,
		client:    client,
		collector: collector,
		labels:    []string{device.Name, device.Address},
	}
}

func (c collectorContext) fields() log.Fields {
	return log.Fields{
		"device":    c.device.Name,
		"collector": c.collector,
	}
}

func (c *collectorContext) withLabels(labels ...string) *collectorContext {
	return &collectorContext{
		ch:        c.ch,
		device:    c.device,
		client:    c.client,
		collector: c.collector,
		labels:    append([]string{c.device.Name, c.device.Address}, labels...),
	}
}
