package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("ppp", newPPPCollector, "retrieves ppp active connections metrics")
}

type pppCollector struct {
	metrics metrics.PropertyMetric
	active  *prometheus.Desc
}

func newPPPCollector() RouterOSCollector {
	const prefix = "ppp"

	// list of labels exposed in metric
	labelNames := []string{"name", "service", "caller_id", "address"}

	return &pppCollector{
		metrics: metrics.NewPropertyConstMetric(prefix, "address", labelNames...).
			WithName("active_peer").
			Build(),
		active: metrics.Description(prefix, "active", "number of active ppp connections",
			metrics.LabelDevName, metrics.LabelDevAddress),
	}
}

func (c *pppCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *pppCollector) Collect(ctx *metrics.CollectorContext) error {
	// do not load entries if not configured
	if ctx.FeatureCfg.BoolValue("details", false) {
		return c.collectDetails(ctx)
	}

	return c.collectStats(ctx)
}

func (c *pppCollector) collectDetails(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/ppp/active/print", "=.proplist=name,service,caller-id,address")
	if err != nil {
		return fmt.Errorf("fetch ppp error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.WithLabelsFromMap(re.Map, "name", "service", "caller-id", "address")

		// collect metrics using context
		if err := c.metrics.Collect(re.Map, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}
	}

	ctx.Ch <- prometheus.MustNewConstMetric(c.active, prometheus.GaugeValue, float64(len(reply.Re)),
		ctx.Device.Name, ctx.Device.Address)

	return errs.ErrorOrNil()
}

func (c *pppCollector) collectStats(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/ppp/active/print", "=count-only=")
	if err != nil {
		return fmt.Errorf("fetch ppp error: %w", err)
	}

	cnt, err := convert.MetricFromString(reply.Done.Map["ret"])
	if err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	ctx.Ch <- prometheus.MustNewConstMetric(c.active, prometheus.GaugeValue, cnt,
		ctx.Device.Name, ctx.Device.Address)

	return nil
}
