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
}

func (c collectorContext) fields() log.Fields {
	return log.Fields{
		"device":    c.device.Name,
		"collector": c.collector,
	}
}
