package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("dude", newDudeCollector, "retrieves dude metrics")
}

type dudeCollector struct {
	metrics metrics.PropertyMetricList
}

func newDudeCollector() RouterOSCollector {
	const prefix = "dude"

	return &dudeCollector{
		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyGaugeMetric(prefix, "enabled").WithConverter(convert.MetricFromBool).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "status").WithName("running").WithConverter(dudeStatusParser).Build(),
		},
	}
}

func (c *dudeCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *dudeCollector) Collect(ctx *metrics.CollectorContext) error {
	// list of props must contain all values for labels and metrics
	reply, err := ctx.Client.Run("/dude/print", "?disabled=false", "=.proplist=enabled,status")
	if err != nil {
		return fmt.Errorf("fetch dude error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		if err := c.metrics.Collect(re.Map, ctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}

func dudeStatusParser(inp string) (float64, error) {
	if inp == "running" {
		return 1.0, nil
	}

	return 0.0, nil
}
