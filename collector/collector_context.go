package collector

import (
	"mikrotik-exporter/config"

	routeros "github.com/KarolBedkowski/routeros-go-client"
	"github.com/prometheus/client_golang/prometheus"
)

type collectorContext struct {
	ch     chan<- prometheus.Metric
	device *config.Device
	client *routeros.Client
}
