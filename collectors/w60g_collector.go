package collectors

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

// TODO: need check

func init() {
	registerCollector("w60g", neww60gInterfaceCollector,
		"retrieves W60G interface metrics")
}

type w60gInterfaceCollector struct {
	metrics propertyMetricList
}

func neww60gInterfaceCollector() RouterOSCollector {
	const prefix = "w60ginterface"

	labelNames := []string{"name", "address", "interface"}

	return &w60gInterfaceCollector{
		metrics: propertyMetricList{
			newPropertyGaugeMetric(prefix, "signal", labelNames).build(),
			newPropertyGaugeMetric(prefix, "rssi", labelNames).build(),
			newPropertyGaugeMetric(prefix, "tx-mcs", labelNames).build(),
			newPropertyGaugeMetric(prefix, "frequency", labelNames).build(),
			newPropertyGaugeMetric(prefix, "tx-phy-rate", labelNames).build(),
			newPropertyGaugeMetric(prefix, "tx-sector", labelNames).build(),
			newPropertyGaugeMetric(prefix, "distance", labelNames).build(),
			newPropertyGaugeMetric(prefix, "tx-packet-error-rate", labelNames).build(),
		},
	}
}

func (c *w60gInterfaceCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.describe(ch)
}

func (c *w60gInterfaceCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/interface/w60g/print", "=.proplist=name")
	if err != nil {
		return fmt.Errorf("fetch w60g error: %w", err)
	}

	ifaces := extractPropertyFromReplay(reply, "name")
	if len(ifaces) == 0 {
		return nil
	}

	return c.collectw60gMetricsForInterfaces(ifaces, ctx)
}

func (c *w60gInterfaceCollector) collectw60gMetricsForInterfaces(ifaces []string, ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/interface/w60g/monitor",
		"=numbers="+strings.Join(ifaces, ","),
		"=once=",
		"=.proplist=name,signal,rssi,tx-mcs,frequency,tx-phy-rate,tx-sector,distance,tx-packet-error-rate")
	if err != nil {
		return fmt.Errorf("fetch w60g monitor error: %w", err)
	}

	var errs *multierror.Error

	for _, se := range reply.Re {
		if name, ok := se.Map["name"]; ok {
			ctx = ctx.withLabels(name)

			if err := c.metrics.collect(se, ctx); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("collect %v error: %w", name, err))
			}
		}
	}

	return errs.ErrorOrNil()
}
