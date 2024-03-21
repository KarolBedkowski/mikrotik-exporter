package collector

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("health", newhealthCollector)
}

type healthCollector struct {
	metrics propertyMetricList
}

func newhealthCollector() routerOSCollector {
	const prefix = "health"

	labelNames := []string{"name", "address"}

	return &healthCollector{
		metrics: propertyMetricList{
			newPropertyGaugeMetric(prefix, "voltage", labelNames).
				withHelp("Input voltage to the RouterOS board, in volts").build(),
			newPropertyGaugeMetric(prefix, "temperature", labelNames).
				withHelp("Temperature of RouterOS board, in degrees Celsius").build(),
			newPropertyGaugeMetric(prefix, "cpu-temperature", labelNames).
				withHelp("Temperature of RouterOS CPU, in degrees Celsius").build(),
		},
	}
}

func (c *healthCollector) describe(ch chan<- *prometheus.Desc) {
	c.metrics.describe(ch)
}

func (c *healthCollector) collect(ctx *collectorContext) error {
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

		if err := c.metrics.collect(re, ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}
