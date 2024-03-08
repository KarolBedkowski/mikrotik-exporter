package collector

import (
	"mikrotik-exporter/config"

	"github.com/prometheus/client_golang/prometheus"
	routeros "mikrotik-exporter/routeros"
)

type collectorContext struct {
	ch     chan<- prometheus.Metric
	device *config.Device
	client *routeros.Client
}
