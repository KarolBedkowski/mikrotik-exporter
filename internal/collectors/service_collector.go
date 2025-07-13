package collectors

// Collect info about connections from /ip/service

import (
	"fmt"

	"mikrotik-exporter/internal/metrics"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("service", newServiceConnCollector,
		"retrieves service connections metrics")
}

type serviceConnCollector struct {
	metrics *prometheus.Desc
}

func newServiceConnCollector() RouterOSCollector {
	const prefix = "service"

	return &serviceConnCollector{
		metrics: metrics.Description(prefix, "active_connections_count", "number of active connection for service",
			metrics.LabelDevName, metrics.LabelDevAddress, "service"),
	}
}

func (c *serviceConnCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.metrics
}

func (c *serviceConnCollector) Collect(ctx *metrics.CollectorContext) error {
	if ctx.Device.FirmwareVersion.Compare(7, 19, 0) < 0 { //nolint:mnd
		return NotSupportedError("service collector is available since RO 7.19")
	}

	reply, err := ctx.Client.Run("/ip/service/print", "?connection", "=.proplist=name")
	if err != nil {
		return fmt.Errorf("fetch service stats error: %w", err)
	}

	counter := metrics.CountByProperty(reply.Re, "name")
	for service, count := range counter {
		ctx.Ch <- prometheus.MustNewConstMetric(c.metrics, prometheus.GaugeValue,
			float64(count), ctx.Device.Name, ctx.Device.Address, service)
	}

	return nil
}
