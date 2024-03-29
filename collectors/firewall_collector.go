package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("firewall", newFirewallCollector,
		"retrieves firewall metrics")
}

type firewallCollector struct {
	metrics PropertyMetricList
}

func newFirewallCollector() RouterOSCollector {
	const prefix = "firewall"

	labelNames := []string{"name", "address", "firewall", "chain", "comment"}

	return &firewallCollector{
		metrics: PropertyMetricList{
			NewPropertyCounterMetric(prefix, "packets", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "bytes", labelNames).Build(),
		},
	}
}

func (c *firewallCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *firewallCollector) Collect(ctx *CollectorContext) error {
	var errs *multierror.Error

	for fw, chains := range ctx.device.FWCollectorSettings {
		for _, chain := range chains {
			errs = multierror.Append(nil,
				c.collectStats(fw, chain, ctx))
		}
	}

	return errs.ErrorOrNil()
}

func (c *firewallCollector) collectStats(firewall, chain string, ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/ip/firewall/"+firewall+"/print",
		"=stats=", "=chain="+chain, "?disabled=no",
		"=.proplist=comment,bytes,packets")
	if err != nil {
		return fmt.Errorf("fetch fw stats %s/%s error: %w", firewall, chain, err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		if comment := re.Map["comment"]; comment != "" {
			lctx := ctx.withLabels(firewall, chain, comment)

			if err := c.metrics.Collect(re, &lctx); err != nil {
				errs = multierror.Append(errs,
					fmt.Errorf("collect fw %s/%s error: %w", firewall, chain, err))
			}
		}
	}

	return errs.ErrorOrNil()
}
