package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("interface", newInterfaceCollector, "retrieves interfaces metrics")
}

type interfaceCollector struct {
	metrics metrics.PropertyMetricList
}

func newInterfaceCollector() RouterOSCollector {
	const prefix = "interface"

	labelNames := []string{metrics.LabelInterface, "type", metrics.LabelComment, "slave"}

	return &interfaceCollector{
		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyGaugeMetric(prefix, "actual-mtu", labelNames...).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "running", labelNames...).WithConverter(convert.MetricFromBool).Build(),
			metrics.NewPropertyCounterMetric(prefix, "rx-byte", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "tx-byte", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "rx-packet", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "tx-packet", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "rx-error", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "tx-error", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "rx-drop", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "tx-drop", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "link-downs", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "tx-queue-drop", labelNames...).Build(),
		},
	}
}

func (c *interfaceCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *interfaceCollector) Collect(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/interface/print",
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

		lctx := ctx.WithLabelsFromMap(re.Map, "name", "type", "comment", "slave")

		if err := c.metrics.Collect(re.Map, &lctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}
