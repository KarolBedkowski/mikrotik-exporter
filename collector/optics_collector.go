package collector

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
	metrics propertyMetricList
}

func newOpticsCollector() routerOSCollector {
	const prefix = "optics"

	labelNames := []string{"name", "address", "interface"}

	return &opticsCollector{
		metrics: propertyMetricList{
			newPropertyGaugeMetric(prefix, "sfp-rx-loss", labelNames).
				withHelp("RX status").withConverter(metricFromBool).build(),
			newPropertyGaugeMetric(prefix, "sfp-tx-fault", labelNames).
				withHelp("TX status").withConverter(metricFromBool).build(),
			newPropertyGaugeMetric(prefix, "sfp-rx-power", labelNames).
				withHelp("RX power in dBM").build(),
			newPropertyGaugeMetric(prefix, "sfp-tx-power", labelNames).
				withHelp("TX power in dBM").build(),
			newPropertyGaugeMetric(prefix, "sfp-temperature", labelNames).
				withHelp("temperature in degree celsius").build(),
			newPropertyGaugeMetric(prefix, "sfp-tx-bias-current", labelNames).
				withHelp("bias is milliamps").build(),
			newPropertyGaugeMetric(prefix, "sfp-supply-voltage", labelNames).build(),
		},
	}
}

func (c *opticsCollector) describe(ch chan<- *prometheus.Desc) {
	c.metrics.describe(ch)
}

func (c *opticsCollector) collect(ctx *collectorContext) error {
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

func (c *opticsCollector) collectOpticalMetricsForInterfaces(ifaces []string, ctx *collectorContext) error {
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
			ctx = ctx.withLabels(name)

			if err := c.metrics.collect(se, ctx); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("collect %s error: %w", name, err))
			}
		}
	}

	return errs.ErrorOrNil()
}
