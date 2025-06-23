package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("arp", newARPCollector, "retrieves arp metrics")
}

type arpCollector struct {
	metrics  metrics.PropertyMetric
	statuses metrics.PropertyMetric
	invalid  metrics.PropertyMetric

	statusesNames []string
}

func newARPCollector() RouterOSCollector {
	const prefix = "arp"

	// list of labels exposed in metric
	labelNames := []string{
		"client_address", metrics.LabelInterface, "mac_address", metrics.LabelComment, "dhcp",
		"dynamic",
	}

	return &arpCollector{
		// get metric `arp_complete` with value loaded from `complete` property converted to 1/0
		// and with `labelNames`.
		metrics: metrics.NewPropertyGaugeMetric(prefix, "complete", labelNames...).
			WithConverter(convert.MetricFromBool).
			Build(),
		statuses: metrics.NewPropertyRetMetric(prefix, "status", "status").Build(),
		invalid:  metrics.NewPropertyRetMetric(prefix, "invalid").Build(),

		statusesNames: []string{"delay", "failed", "incomplete", "permanent", "probe", "reachable", "stale"},
	}
}

func (c *arpCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
	c.statuses.Describe(ch)
	c.invalid.Describe(ch)
}

func (c *arpCollector) Collect(ctx *metrics.CollectorContext) error {
	return multierror.Append(nil,
		c.collectEntries(ctx),
		c.collectStatuses(ctx),
		c.collectInvalid(ctx),
	).ErrorOrNil()
}

func (c *arpCollector) collectEntries(ctx *metrics.CollectorContext) error {
	// list of props must contain all values for labels and metrics
	reply, err := ctx.Client.Run("/ip/arp/print",
		"?complete=true",
		"=.proplist=address,mac-address,interface,comment,dynamic,dhcp,complete")
	if err != nil {
		return fmt.Errorf("fetch arp error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		// create context with labels from reply
		lctx := ctx.WithLabelsFromMap(re.Map, "address", "interface", "mac-address",
			"comment", "dhcp", "dynamic")

		// collect metrics using context
		if err := c.metrics.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}

func (c *arpCollector) collectStatuses(ctx *metrics.CollectorContext) error {
	var errs *multierror.Error

	for _, status := range c.statusesNames {
		reply, err := ctx.Client.Run("/ip/arp/print", "?disabled=false", "?status="+status, "=count-only=")
		if err != nil {
			return fmt.Errorf("fetch arp status %q  error: %w", status, err)
		}

		lctx := ctx.WithLabels(status)

		if err := c.statuses.Collect(reply.Done, &lctx); err != nil {
			return fmt.Errorf("collect arp status %q error: %w", status, err)
		}
	}

	return errs.ErrorOrNil()
}

func (c *arpCollector) collectInvalid(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/ip/arp/print", "?disabled=false", "?invalid=true", "=count-only=")
	if err != nil {
		return fmt.Errorf("fetch arp invalid cnt  error: %w", err)
	}

	if err := c.invalid.Collect(reply.Done, ctx); err != nil {
		return fmt.Errorf("collect arp invalid cnt error: %w", err)
	}

	return nil
}
