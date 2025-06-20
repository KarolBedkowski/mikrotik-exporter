package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("ip", newIPCollector, "retrieves ip metrics")
}

type ipCollector struct {
	metrics PropertyMetricList
}

func newIPCollector() RouterOSCollector {
	const prefix = "ip"

	// list of labels exposed in metric
	labelNames := []string{"name", "address"}

	return &ipCollector{
		metrics: PropertyMetricList{
			NewPropertyCounterMetric(prefix, "ipv4-fast-path-active", labelNames).
				WithConverter(metricFromBool).
				Build(),
			NewPropertyCounterMetric(prefix, "ipv4-fast-path-bytes", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "ipv4-fast-path-packets", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "ipv4-fasttrack-active", labelNames).
				WithConverter(metricFromBool).
				Build(),
			NewPropertyCounterMetric(prefix, "ipv4-fasttrack-bytes", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "ipv4-fasttrack-packets", labelNames).Build(),
		},
	}
}

func (c *ipCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *ipCollector) Collect(ctx *CollectorContext) error {
	// list of props must contain all values for labels and metrics
	reply, err := ctx.client.Run("/ip/settings/print",
		"=.proplist=ipv4-fast-path-active,ipv4-fast-path-bytes,ipv4-fast-path-packets,"+
			"ipv4-fasttrack-active,ipv4-fasttrack-bytes,ipv4-fasttrack-packets")
	if err != nil {
		return fmt.Errorf("fetch ip settings error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		// collect metrics using context
		if err := c.metrics.Collect(re, ctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}
