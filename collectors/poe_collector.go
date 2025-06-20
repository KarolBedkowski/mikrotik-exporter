package collectors

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("poe", newPOECollector, "retrieves POE metrics")
}

type poeCollector struct {
	metrics PropertyMetricList
}

func newPOECollector() RouterOSCollector {
	const prefix = "poe"

	labelNames := []string{"name", "address", "interface"}

	return &poeCollector{
		metrics: PropertyMetricList{
			NewPropertyGaugeMetric(prefix, "current", labelNames).WithHelp("current in mA").Build(),
			NewPropertyGaugeMetric(prefix, "wattage", labelNames).WithHelp("power in W").Build(),
			NewPropertyGaugeMetric(prefix, "voltage", labelNames).WithHelp("voltage in V").Build(),
		},
	}
}

func (c *poeCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *poeCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/interface/ethernet/poe/print", "=.proplist=name")
	if err != nil {
		return fmt.Errorf("fetch ethernet poe error: %w", err)
	}

	if len(reply.Re) == 0 {
		return nil
	}

	ifaces := make([]string, 0, len(reply.Re))
	for _, iface := range reply.Re {
		ifaces = append(ifaces, iface.Map["name"])
	}

	return c.collectPOEMetricsForInterfaces(ifaces, ctx)
}

func (c *poeCollector) collectPOEMetricsForInterfaces(ifaces []string, ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/interface/ethernet/poe/monitor",
		"=numbers="+strings.Join(ifaces, ","), "=once=",
		"=.proplist=poe-out-current,poe-out-voltage,poe-out-power")
	if err != nil {
		return fmt.Errorf("fetch poe monitor for %v error: %w", ifaces, err)
	}

	var errs *multierror.Error

	for _, se := range reply.Re {
		if name, ok := se.Map["name"]; ok {
			lctx := ctx.withLabels(name)

			if err := c.metrics.Collect(se, &lctx); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("collect %v error: %w", name, err))
			}
		}
	}

	return errs.ErrorOrNil()
}
