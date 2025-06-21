package collectors

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("cloud", newCloudCollector, "retrieves cloud services information")
}

type cloudCollector struct {
	ifaceStatus PropertyMetric
}

func newCloudCollector() RouterOSCollector {
	return &cloudCollector{
		// create metrics with postfix and set it to value 1 or 0 according to `status` property.
		ifaceStatus: NewPropertyConstMetric("cloud", "status", "status").Build(),
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
		return UnexpectedResponseError{"get cloud returned more than 1 record", reply}
	}

	re := reply.Re[0]
	lctx := ctx.withLabels(re.Map["status"])

	if err := c.ifaceStatus.Collect(re, &lctx); err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	return nil
}
