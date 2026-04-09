package collectors

import (
	"errors"
	"fmt"

	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/internal/metrics"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("container", newContainerCollector, "retrieves container metrics")
}

type container struct {
	metrics metrics.PropertyMetric
}

func newContainerCollector() RouterOSCollector {
	const prefix = "container"

	labelNames := []string{"name"}

	return &container{
		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyGaugeMetric(prefix, "running", labelNames...).WithDefault("false").
				WithConverter(convert.MetricFromBool).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "starting", labelNames...).WithDefault("false").
				WithConverter(convert.MetricFromBool).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "unhealthy", labelNames...).WithDefault("false").
				WithConverter(convert.MetricFromBool).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "stopped", labelNames...).WithDefault("false").
				WithConverter(convert.MetricFromBool).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "stopping", labelNames...).WithDefault("false").
				WithConverter(convert.MetricFromBool).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "healthy", labelNames...).WithDefault("false").
				WithConverter(convert.MetricFromBool).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "memory-current", labelNames...).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "cpu-usage", labelNames...).Build(),
		},
	}
}

func (c *container) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *container) Collect(ctx *metrics.CollectorContext) error {
	if ctx.Device.FirmwareVersion.Compare(7, 5, 0) < 0 && //nolint:mnd
		ctx.Device.FirmwareVersion.CheckArchitecture("x86_64", "arm", "arm64") {
		return NotSupportedError("container")
	}

	reply, err := ctx.Client.Run("/container/print",
		"=.proplist=name,running,starting,unhealthy,stopped,stopping,healthy,memory-current,cpu-usage")
	if err != nil {
		return fmt.Errorf("fetch container error: %w", err)
	}

	ctx.Logger.Debug("collect", "reply", reply)

	var errs error

	for _, re := range reply.Re {
		lctx := ctx.WithLabelsFromMap(re.Map, "name")

		if err := c.metrics.Collect(re.Map, &lctx); err != nil {
			errs = errors.Join(errs, fmt.Errorf("collect error: %w", err))
		}
	}

	return errs
}
