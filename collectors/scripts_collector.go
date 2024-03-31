package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("scripts", newScriptCollector,
		"retrieves metrics from scripts")
}

type scriptCollector struct {
	metric *prometheus.Desc
}

func newScriptCollector() RouterOSCollector {
	labelNames := []string{"name", "address", "script"}

	return &scriptCollector{
		metric: description("", "script", "metrics from scripts", labelNames),
	}
}

func (c *scriptCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.metric
}

func (c *scriptCollector) Collect(ctx *CollectorContext) error {
	var errs *multierror.Error
	for _, script := range ctx.device.Scripts {
		errs = multierror.Append(errs, c.collectScript(ctx, script))
	}

	return errs.ErrorOrNil()
}

func (c *scriptCollector) collectScript(ctx *CollectorContext, script string) error {
	reply, err := ctx.client.Run("/system/script/run", "=number="+script)
	if err != nil {
		return fmt.Errorf("run script %s error: %w", script, err)
	}

	if v, ok := reply.Done.Map["ret"]; ok {
		value, err := metricFromString(v)
		if err != nil {
			return fmt.Errorf("parse script %s result %v error: %w", script, v, err)
		}

		ctx.ch <- prometheus.MustNewConstMetric(c.metric, prometheus.GaugeValue,
			value, ctx.device.Name, ctx.device.Address, script)
	}

	return nil
}
