package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("arp", newARPCollector, "retrieves arp metrics")
}

type arpCollector struct {
	metrics  metrics.PropertyMetricList
	statuses metrics.PropertyMetric
}

func newARPCollector() RouterOSCollector {
	const prefix = "arp"

	// list of labels exposed in metric
	labelNames := []string{"client_address", metrics.LabelInterface, "mac_address", metrics.LabelComment}
	statusLabelNames := []string{"client_address", metrics.LabelInterface, "mac_address", metrics.LabelComment, "status"}

	return &arpCollector{
		metrics: metrics.PropertyMetricList{
			// get mac-address but rename metric to <prefix>_entry, apply labels and use constant value (1)
			// for all entries
			metrics.NewPropertyConstMetric(prefix, "mac-address", labelNames...).WithName("entry").Build(),
			// get `dynamic` value, convert this bool value to 1/0
			metrics.NewPropertyGaugeMetric(prefix, "dynamic", labelNames...).WithConverter(convert.MetricFromBool).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "dhcp", labelNames...).WithConverter(convert.MetricFromBool).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "invalid", labelNames...).WithConverter(convert.MetricFromBool).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "published", labelNames...).WithConverter(convert.MetricFromBool).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "complete", labelNames...).WithConverter(convert.MetricFromBool).Build(),
		},
		statuses: metrics.NewPropertyConstMetric(prefix, "mac-address", statusLabelNames...).WithName("status").Build(),
	}
}

func (c *arpCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *arpCollector) Collect(ctx *metrics.CollectorContext) error {
	// list of props must contain all values for labels and metrics
	reply, err := ctx.Client.Run("/ip/arp/print",
		"?disabled=false",
		"=.proplist=address,mac-address,interface,comment,dynamic,dhcp,complete,status,"+
			"invalid,published")
	if err != nil {
		return fmt.Errorf("fetch arp error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		// create context with labels from reply
		lctx := ctx.WithLabelsFromMap(re.Map, "address", "interface", "mac-address", "comment")

		// collect metrics using context
		if err := c.metrics.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}

		// add additional label to existing labels and collect data
		lctx = lctx.AppendLabelsFromMap(re.Map, "status")
		if err := c.statuses.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}
