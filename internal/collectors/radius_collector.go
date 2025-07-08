package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/metrics"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("radius", newRadiusCollector,
		"retrieves Radius incoming metrics")
}

type radiusCollector struct {
	metrics metrics.PropertyMetric
}

func newRadiusCollector() RouterOSCollector {
	const prefix = "radius_incoming"

	return &radiusCollector{
		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyCounterMetric(prefix, "requests").Build(),
			metrics.NewPropertyCounterMetric(prefix, "bad-requests").Build(),
			metrics.NewPropertyCounterMetric(prefix, "naks").Build(),
			metrics.NewPropertyCounterMetric(prefix, "acks").Build(),
		},
	}
}

func (c *radiusCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *radiusCollector) Collect(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/radius/incoming/monitor", "=once=")
	if err != nil {
		return fmt.Errorf("fetch radius incoming monitor error: %w", err)
	}

	if len(reply.Re) == 0 {
		return nil
	}

	re := reply.Re[0]

	if err := c.metrics.Collect(re.Map, ctx); err != nil {
		return fmt.Errorf("collect radius incoming monitor error: %w", err)
	}

	return nil
}
