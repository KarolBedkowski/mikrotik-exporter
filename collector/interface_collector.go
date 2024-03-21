package collector

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("interface", newInterfaceCollector)
}

type interfaceCollector struct {
	metrics propertyMetricList
}

func newInterfaceCollector() routerOSCollector {
	const prefix = "interface"

	labelNames := []string{"name", "address", "interface", "type", "disabled", "comment", "running", "slave"}

	return &interfaceCollector{
		metrics: propertyMetricList{
			newPropertyGaugeMetric(prefix, "actual-mtu", labelNames).build(),
			newPropertyGaugeMetric(prefix, "running", labelNames).withConverter(metricFromBool).build(),
			newPropertyCounterMetric(prefix, "rx-byte", labelNames).build(),
			newPropertyCounterMetric(prefix, "tx-byte", labelNames).build(),
			newPropertyCounterMetric(prefix, "rx-packet", labelNames).build(),
			newPropertyCounterMetric(prefix, "tx-packet", labelNames).build(),
			newPropertyCounterMetric(prefix, "rx-error", labelNames).build(),
			newPropertyCounterMetric(prefix, "tx-error", labelNames).build(),
			newPropertyCounterMetric(prefix, "rx-drop", labelNames).build(),
			newPropertyCounterMetric(prefix, "tx-drop", labelNames).build(),
			newPropertyCounterMetric(prefix, "link-downs", labelNames).build(),
		},
	}
}

func (c *interfaceCollector) describe(ch chan<- *prometheus.Desc) {
	c.metrics.describe(ch)
}

func (c *interfaceCollector) collect(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/interface/print",
		"=.proplist=name,type,disabled,comment,slave,actual-mtu,running,rx-byte,tx-byte,"+
			"rx-packet,tx-packet,rx-error,tx-error,rx-drop,tx-drop,link-downs")
	if err != nil {
		return fmt.Errorf("fetch interfaces detail error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		ctx = ctx.withLabels(
			re.Map["name"], re.Map["type"], re.Map["disabled"],
			re.Map["comment"], re.Map["running"], re.Map["slave"],
		)

		if err := c.metrics.collect(re, ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}
