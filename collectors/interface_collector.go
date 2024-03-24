package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("interface", newInterfaceCollector, "retrieves interfaces metrics")
}

type interfaceCollector struct {
	metrics PropertyMetricList
}

func newInterfaceCollector() RouterOSCollector {
	const prefix = "interface"

	labelNames := []string{"name", "address", "interface", "type", "disabled", "comment", "running", "slave"}

	return &interfaceCollector{
		metrics: PropertyMetricList{
			NewPropertyGaugeMetric(prefix, "actual-mtu", labelNames).Build(),
			NewPropertyGaugeMetric(prefix, "running", labelNames).WithConverter(metricFromBool).Build(),
			NewPropertyCounterMetric(prefix, "rx-byte", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "tx-byte", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "rx-packet", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "tx-packet", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "rx-error", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "tx-error", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "rx-drop", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "tx-drop", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "link-downs", labelNames).Build(),
		},
	}
}

func (c *interfaceCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *interfaceCollector) Collect(ctx *CollectorContext) error {
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

		if err := c.metrics.Collect(re, ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}
