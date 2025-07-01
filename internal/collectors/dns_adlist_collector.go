package collectors

//
// Get metrics from /dns/adlist - number of entries and total match for each adlist.

import (
	"fmt"

	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("dns_adlist", newDNSAdlistCollector, "retrieves dns adlist metrics")
}

type dnsAdlistCollector struct {
	metrics metrics.PropertyMetricList
}

func newDNSAdlistCollector() RouterOSCollector {
	const prefix = "dns_adlist"

	return &dnsAdlistCollector{
		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyCounterMetric(prefix, "match-count", "url").WithName("match_count_total").Build(),
			metrics.NewPropertyGaugeMetric(prefix, "name-count", "url").WithName("name_count").Build(),
		},
	}
}

func (c *dnsAdlistCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *dnsAdlistCollector) Collect(ctx *metrics.CollectorContext) error {
	if ctx.Device.FirmwareVersion.Compare(7, 15, 0) < 0 { //nolint:mnd
		return NotSupportedError("dns_adlist")
	}

	reply, err := ctx.Client.Run("/ip/dns/adlist/print",
		"?disabled=false",
		"=.proplist=url,match-count,name-count")
	if err != nil {
		return fmt.Errorf("fetch dns adlist stats error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.WithLabelsFromMap(re.Map, "url")
		if err := c.metrics.Collect(re.Map, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}
