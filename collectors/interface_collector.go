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

	labelNames := []string{"name", "address", "interface", "type", "comment", "slave"}

	return &interfaceCollector{
		metrics: PropertyMetricList{
			NewPropertyGaugeMetric(prefix, "actual-mtu", labelNames).Build(),
			NewPropertyGaugeMetric(prefix, "running", labelNames).WithConverter(metricFromBool).Build(),
			NewPropertyGaugeMetric(prefix, "disabled", labelNames).WithConverter(metricFromBool).Build(),
			NewPropertyCounterMetric(prefix, "rx-byte", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "tx-byte", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "rx-packet", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "tx-packet", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "rx-error", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "tx-error", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "rx-drop", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "tx-drop", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "link-downs", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "tx-queue-drop", labelNames).Build(),
		},
	}
}

func (c *interfaceCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *interfaceCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/interface/print",
		"?disabled=false",
		"=.proplist=name,type,disabled,comment,slave,actual-mtu,running,rx-byte,tx-byte,"+
			"rx-packet,tx-packet,rx-error,tx-error,rx-drop,tx-drop,link-downs,tx-queue-drop")
	if err != nil {
		return fmt.Errorf("fetch interfaces detail error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		if re.Map["name"] == "lo" {
			continue
		}

		lctx := ctx.withLabelsFromMap(re.Map, "name", "type", "comment", "slave")

		if err := c.metrics.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}
