package collectors

import (
	"fmt"
	"mikrotik-exporter/internal/metrics"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

// TODO: need check

func init() {
	registerCollector("optics", newOpticsCollector,
		"retrieves optical diagnostic metrics")
}

type opticsCollector struct {
	metrics metrics.PropertyMetricList
}

func newOpticsCollector() RouterOSCollector {
	const prefix = "optics"

	return &opticsCollector{
		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyGaugeMetric(prefix, "sfp-rx-loss", metrics.LabelInterface).
				WithHelp("RX status").
				WithConverter(metrics.MetricFromBool).
				Build(),
			metrics.NewPropertyGaugeMetric(prefix, "sfp-tx-fault", metrics.LabelInterface).
				WithHelp("TX status").
				WithConverter(metrics.MetricFromBool).
				Build(),
			metrics.NewPropertyGaugeMetric(prefix, "sfp-rx-power", metrics.LabelInterface).
				WithHelp("RX power in dBM").
				Build(),
			metrics.NewPropertyGaugeMetric(prefix, "sfp-tx-power", metrics.LabelInterface).
				WithHelp("TX power in dBM").
				Build(),
			metrics.NewPropertyGaugeMetric(prefix, "sfp-temperature", metrics.LabelInterface).
				WithHelp("temperature in degree celsius").
				Build(),
			metrics.NewPropertyGaugeMetric(prefix, "sfp-tx-bias-current", metrics.LabelInterface).
				WithHelp("bias is milliamps").
				Build(),
			metrics.NewPropertyGaugeMetric(prefix, "sfp-supply-voltage", metrics.LabelInterface).Build(),
		},
	}
}

func (c *opticsCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *opticsCollector) Collect(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/interface/ethernet/print",
		"?disabled=false",
		"=.proplist=name,default-name")
	if err != nil {
		return fmt.Errorf("fetch ethernet error: %w", err)
	}

	if len(reply.Re) == 0 {
		return nil
	}

	ifaces := make([]string, 0, len(reply.Re))

	for _, iface := range reply.Re {
		n := iface.Map["name"]
		if strings.HasPrefix(n, "sfp") || strings.HasPrefix(iface.Map["default-name"], "sfp") {
			ifaces = append(ifaces, n)
		}
	}

	return c.collectOpticalMetricsForInterfaces(ifaces, ctx)
}

func (c *opticsCollector) collectOpticalMetricsForInterfaces(ifaces []string, ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/interface/ethernet/monitor",
		"=numbers="+strings.Join(ifaces, ","),
		"=once=",
		"=.proplist=name,sfp-rx-loss,sfp-tx-fault,sfp-temperature,sfp-supply-voltage,sfp-rx-power,"+
			"sfp-tx-power,sfp-tx-bias-current")
	if err != nil {
		return fmt.Errorf("fetch ethernet monitor for %v error: %w", ifaces, err)
	}

	var errs *multierror.Error

	for _, se := range reply.Re {
		if name, ok := se.Map["name"]; ok {
			lctx := ctx.WithLabels(name)

			if err := c.metrics.Collect(se, &lctx); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("collect %s error: %w", name, err))
			}
		}
	}

	return errs.ErrorOrNil()
}
