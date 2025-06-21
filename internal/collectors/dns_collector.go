package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("dns", newDNSCollector, "retrieves dns metrics")
}

type dnsCollector struct {
	metrics metrics.PropertyMetricList
}

func newDNSCollector() RouterOSCollector {
	const prefix = "dns"

	return &dnsCollector{
		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyGaugeMetric(prefix, "cache-used").WithName("cache_used_bytes").Build(),
			metrics.NewPropertyGaugeMetric(prefix, "cache-size").WithName("cache_size_bytes").Build(),
		},
	}
}

func (c *dnsCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *dnsCollector) Collect(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/ip/dns/print", "=.proplist=cache-size,cache-used")
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
