package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("health", newhealthCollector, "retrieves board Health metrics")
}

type healthCollector struct {
	metrics PropertyMetricList
}

func newhealthCollector() RouterOSCollector {
	const prefix = "health"

	labelNames := []string{"name", "address"}

	return &healthCollector{
		metrics: PropertyMetricList{
			NewPropertyGaugeMetric(prefix, "voltage", labelNames).
				WithHelp("Input voltage to the RouterOS board, in volts").Build(),
			NewPropertyGaugeMetric(prefix, "temperature", labelNames).
				WithHelp("Temperature of RouterOS board, in degrees Celsius").Build(),
			NewPropertyGaugeMetric(prefix, "cpu-temperature", labelNames).
				WithHelp("Temperature of RouterOS CPU, in degrees Celsius").Build(),
		},
	}
}

func (c *healthCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *healthCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/system/health/print")
	if err != nil {
		return fmt.Errorf("fetch health error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		if metric, ok := re.Map["name"]; ok {
			if v, ok := re.Map["value"]; ok {
				re.Map[metric] = v
			} else {
				continue
			}
		}

		if err := c.metrics.Collect(re, ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}
