package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/metrics"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("cloud", newCloudCollector, "retrieves cloud services information")
}

type cloudCollector struct {
	ifaceStatus metrics.PropertyMetric
}

func newCloudCollector() RouterOSCollector {
	return &cloudCollector{
		// create metrics with postfix and set it to value 1 or 0 according to `status` property.
		ifaceStatus: metrics.NewPropertyConstMetric("cloud", "status", "status").Build(),
	}
}

func (c *cloudCollector) Describe(ch chan<- *prometheus.Desc) {
	c.ifaceStatus.Describe(ch)
}

func (c *cloudCollector) Collect(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/ip/cloud/print")
	if err != nil {
		return fmt.Errorf("get cloud error: %w", err)
	}

	if len(reply.Re) != 1 {
		return UnexpectedResponseError{"get cloud returned more than 1 record", reply}
	}

	re := reply.Re[0]
	lctx := ctx.WithLabels(re.Map["status"])

	if err := c.ifaceStatus.Collect(re.Map, &lctx); err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	return nil
}
