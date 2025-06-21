package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/internal/metrics"

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
	return &scriptCollector{
		metric: metrics.Description("", "script", "metrics from scripts",
			metrics.LabelDevName, metrics.LabelDevAddress, "script"),
	}
}

func (c *scriptCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.metric
}

func (c *scriptCollector) Collect(ctx *metrics.CollectorContext) error {
	var errs *multierror.Error
	for _, script := range ctx.Device.Scripts {
		errs = multierror.Append(errs, c.collectScript(ctx, script))
	}

	return errs.ErrorOrNil()
}

func (c *scriptCollector) collectScript(ctx *metrics.CollectorContext, script string) error {
	reply, err := ctx.Client.Run("/system/script/run", "=number="+script)
	if err != nil {
		return fmt.Errorf("run script %s error: %w", script, err)
	}

	if v, ok := reply.Done.Map["ret"]; ok {
		value, err := convert.MetricFromString(v)
		if err != nil {
			return fmt.Errorf("parse script %s result %v error: %w", script, v, err)
		}

		ctx.Ch <- prometheus.MustNewConstMetric(c.metric, prometheus.GaugeValue,
			value, ctx.Device.Name, ctx.Device.Address, script)
	}

	return nil
}
