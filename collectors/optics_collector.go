package collectors

import (
	"fmt"
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
	metrics PropertyMetricList
}

func newOpticsCollector() RouterOSCollector {
	const prefix = "optics"

	labelNames := []string{"name", "address", "interface"}

	return &opticsCollector{
		metrics: PropertyMetricList{
			NewPropertyGaugeMetric(prefix, "sfp-rx-loss", labelNames).
				WithHelp("RX status").WithConverter(metricFromBool).Build(),
			NewPropertyGaugeMetric(prefix, "sfp-tx-fault", labelNames).
				WithHelp("TX status").WithConverter(metricFromBool).Build(),
			NewPropertyGaugeMetric(prefix, "sfp-rx-power", labelNames).
				WithHelp("RX power in dBM").Build(),
			NewPropertyGaugeMetric(prefix, "sfp-tx-power", labelNames).
				WithHelp("TX power in dBM").Build(),
			NewPropertyGaugeMetric(prefix, "sfp-temperature", labelNames).
				WithHelp("temperature in degree celsius").Build(),
			NewPropertyGaugeMetric(prefix, "sfp-tx-bias-current", labelNames).
				WithHelp("bias is milliamps").Build(),
			NewPropertyGaugeMetric(prefix, "sfp-supply-voltage", labelNames).Build(),
		},
	}
}

func (c *opticsCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *opticsCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/interface/ethernet/print", "=.proplist=name,default-name")
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

func (c *opticsCollector) collectOpticalMetricsForInterfaces(ifaces []string, ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/interface/ethernet/monitor",
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
			lctx := ctx.withLabels(name)

			if err := c.metrics.Collect(se, &lctx); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("collect %s error: %w", name, err))
			}
		}
	}

	return errs.ErrorOrNil()
}
