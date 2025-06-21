package collectors

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("monitor", newMonitorCollector,
		"retrieves ethernet interfaces monitor metrics")
}

type monitorCollector struct {
	metrics PropertyMetricList
}

func newMonitorCollector() RouterOSCollector {
	const prefix = "monitor"

	return &monitorCollector{
		metrics: PropertyMetricList{
			NewPropertyGaugeMetric(prefix, "status", LabelInterface).WithConverter(metricFromLinkStatus).Build(),
			NewPropertyGaugeMetric(prefix, "rate", LabelInterface).WithConverter(metricFromRate).Build(),
			NewPropertyGaugeMetric(prefix, "full-duplex", LabelInterface).WithConverter(metricFromBool).Build(),
		},
	}
}

func (c *monitorCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *monitorCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/interface/ethernet/print", "?disabled=false", "=.proplist=name")
	if err != nil {
		return fmt.Errorf("fetch ethernet error: %w", err)
	}

	eths := extractPropertyFromReplay(reply, "name")

	return c.collectForMonitor(eths, ctx)
}

func (c *monitorCollector) collectForMonitor(eths []string, ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/interface/ethernet/monitor",
		"=numbers="+strings.Join(eths, ","),
		"=once=",
		"=.proplist=name,status,rate,full-duplex")
	if err != nil {
		return fmt.Errorf("get ethernet monitor error: %w", err)
	}

	var errs *multierror.Error

	for _, e := range reply.Re {
		name := e.Map["name"]
		lctx := ctx.withLabels(name)

		if err := c.metrics.Collect(e, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect %v error: %w", name, err))
		}
	}

	return errs.ErrorOrNil()
}

func metricFromLinkStatus(value string) (float64, error) {
	if value == "link-ok" {
		return 1.0, nil
	}

	return 0.0, nil
}

func metricFromRate(v string) (float64, error) { //nolint:cyclop
	value := 0

	switch v {
	case "10Mbps":
		value = 10
	case "100Mbps":
		value = 100
	case "1Gbps":
		value = 1000
	case "2.5Gbps":
		value = 2500
	case "5Gbps":
		value = 5000
	case "10Gbps":
		value = 10000
	case "25Gbps":
		value = 25000
	case "40Gbps":
		value = 40000
	case "50Gbps":
		value = 50000
	case "100Gbps":
		value = 100000
	}

	return float64(value), nil
}
