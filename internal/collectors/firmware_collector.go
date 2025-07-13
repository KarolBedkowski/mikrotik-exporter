package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("firmware", newFirmwareCollector, "retrieves firmware version")
}

type firmwareCollector struct {
	metric metrics.PropertyMetric
}

func newFirmwareCollector() RouterOSCollector {
	return &firmwareCollector{
		metric: metrics.NewPropertyGaugeMetric("system", "disabled", "name", "version", "build_time").
			WithName("package_enabled").
			WithConverter(convert.MetricFromBoolNeg).
			Build(),
	}
}

func (c *firmwareCollector) Describe(ch chan<- *prometheus.Desc) {
	// ch <- c.description
	c.metric.Describe(ch)
}

func (c *firmwareCollector) Collect(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/system/package/getall")
	if err != nil {
		return fmt.Errorf("fetch package error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.WithLabelsFromMap(re.Map, "name", "version", "build-time")
		if err := c.metric.Collect(re.Map, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect from package error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}
