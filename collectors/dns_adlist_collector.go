package collectors

//
// Get metrics from /dns/adlist - number of entries and total match for each adlist.

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("dns_adlist", newDNSAdlistCollector, "retrieves dns adlist metrics")
}

type dnsAdlistCollector struct {
	metrics PropertyMetricList
}

func newDNSAdlistCollector() RouterOSCollector {
	const prefix = "dns_adlist"

	labelNames := []string{"name", "address", "url"}

	return &dnsAdlistCollector{
		metrics: PropertyMetricList{
			NewPropertyCounterMetric(prefix, "match-count", labelNames).WithName("match_count_total").Build(),
			NewPropertyGaugeMetric(prefix, "name-count", labelNames).WithName("name_count").Build(),
		},
	}
}

func (c *dnsAdlistCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *dnsAdlistCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/ip/dns/adlist/print", "=.proplist=url,match-count,name-count")
	if err != nil {
		return fmt.Errorf("fetch dns adlist stats error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.withLabelsFromMap(re.Map, "url")
		if err := c.metrics.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}
