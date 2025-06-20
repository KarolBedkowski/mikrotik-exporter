package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("arp", newARPCollector,
		"retrieves arp metrics")
}

type arpCollector struct {
	metrics  PropertyMetricList
	statuses PropertyMetric
}

func newARPCollector() RouterOSCollector {
	const prefix = "arp"

	labelNames := []string{"name", "address", "client_address", "interface", "mac_address", "comment"}
	statusLabelNames := []string{"name", "address", "client_address", "interface", "mac_address", "comment", "status"}

	collector := &arpCollector{
		metrics: PropertyMetricList{
			NewPropertyGaugeMetric(prefix, "mac-address", labelNames).WithName("entry").
				WithConverter(metricConstantValue).Build(),
			NewPropertyGaugeMetric(prefix, "dynamic", labelNames).WithConverter(metricFromBool).Build(),
			NewPropertyGaugeMetric(prefix, "dhcp", labelNames).WithConverter(metricFromBool).Build(),
			NewPropertyGaugeMetric(prefix, "invalid", labelNames).WithConverter(metricFromBool).Build(),
			NewPropertyGaugeMetric(prefix, "published", labelNames).WithConverter(metricFromBool).Build(),
			NewPropertyGaugeMetric(prefix, "complete", labelNames).WithConverter(metricFromBool).Build(),
		},
		statuses: NewPropertyGaugeMetric(prefix, "mac-address", statusLabelNames).WithName("status").
			WithConverter(metricConstantValue).Build(),
	}

	return collector
}

func (c *arpCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *arpCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/ip/arp/print",
		"?disabled=false",
		"=.proplist=address,mac-address,interface,comment,dynamic,dhcp,complete,status,"+
			"invalid,published")
	if err != nil {
		return fmt.Errorf("fetch arp error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.withLabelsFromMap(re.Map, "address", "interface", "mac-address", "comment")

		if err := c.metrics.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}

		lctx = lctx.appendLabelsFromMap(re.Map, "status")
		if err := c.statuses.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}
