package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("dns", newDNSCollector, "retrieves dns metrics")
}

type dnsCollector struct {
	metrics PropertyMetricList
}

func newDNSCollector() RouterOSCollector {
	const prefix = "dns"

	labelNames := []string{"name", "address"}

	return &dnsCollector{
		metrics: PropertyMetricList{
			NewPropertyGaugeMetric(prefix, "cache-used", labelNames).WithName("cache_used_bytes").Build(),
			NewPropertyGaugeMetric(prefix, "cache-size", labelNames).WithName("cache_size_bytes").Build(),
		},
	}
}

func (c *dnsCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *dnsCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/ip/dns/print", "=.proplist=cache-size,cache-used")
	if err != nil {
		return fmt.Errorf("fetch dns stats error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		if err := c.metrics.Collect(re, ctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}
