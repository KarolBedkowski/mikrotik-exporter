package collectors

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("cloud", newCloudCollector,
		"retrieves cloud services information")
}

type cloudCollector struct {
	ifaceStatus PropertyMetric
}

func newCloudCollector() RouterOSCollector {
	labelNames := []string{"name", "address"}

	return &cloudCollector{
		ifaceStatus: NewPropertyStatusMetric("cloud", "status", labelNames,
			"unknown", "updated", "updating", "error").Build(),
	}
}

func (c *cloudCollector) Describe(ch chan<- *prometheus.Desc) {
	c.ifaceStatus.Describe(ch)
}

func (c *cloudCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/ip/cloud/print")
	if err != nil {
		return fmt.Errorf("get cloud error: %w", err)
	}

	if len(reply.Re) != 1 {
		return nil
	}

	if err := c.ifaceStatus.Collect(reply.Re[0], ctx); err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	return nil
}
