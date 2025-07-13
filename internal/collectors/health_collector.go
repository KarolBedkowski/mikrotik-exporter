package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("health", newhealthCollector, "retrieves board Health metrics")
}

type healthCollector struct {
	metrics metrics.PropertyMetricList
}

func newhealthCollector() RouterOSCollector {
	const prefix = "health"

	return &healthCollector{
		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyGaugeMetric(prefix, "voltage").
				WithHelp("Input voltage to the RouterOS board, in volts").
				Build(),
			metrics.NewPropertyGaugeMetric(prefix, "temperature").
				WithHelp("Temperature of RouterOS board, in degrees Celsius").
				Build(),
			metrics.NewPropertyGaugeMetric(prefix, "cpu-temperature").
				WithHelp("Temperature of RouterOS CPU, in degrees Celsius").
				Build(),
		},
	}
}

func (c *healthCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *healthCollector) Collect(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/system/health/print")
	if err != nil {
		return fmt.Errorf("fetch health error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		// hack for old ros
		if metric, ok := re.Map["name"]; ok {
			if v, ok := re.Map["value"]; ok {
				re.Map[metric] = v
			} else {
				continue
			}
		}

		if err := c.metrics.Collect(re.Map, ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}
