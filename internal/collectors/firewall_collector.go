package collectors

import (
	"fmt"
	"strings"

	"mikrotik-exporter/internal/config"
	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("firewall", newFirewallCollector, "retrieves firewall metrics")
}

type firewallCollector struct {
	metrics metrics.PropertyMetricList
}

func newFirewallCollector() RouterOSCollector {
	const prefix = "firewall"

	labelNames := []string{"firewall", "chain", metrics.LabelComment}

	return &firewallCollector{
		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyCounterMetric(prefix, "packets", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "bytes", labelNames...).Build(),
		},
	}
}

func (c *firewallCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *firewallCollector) Collect(ctx *metrics.CollectorContext) error {
	var errs *multierror.Error

	fwchains, err := ctx.FeatureCfg.Strs("sources")
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	if len(fwchains) == 0 {
		return config.InvalidConfigurationError("missing configuration")
	}

	for _, fwc := range fwchains {
		fw, chain, found := strings.Cut(fwc, ",")
		if !found {
			errs = multierror.Append(errs,
				config.InvalidConfigurationError(fmt.Sprintf("invalid entry %q, required 'firewall,chain'", fwc)))

			continue
		}

		fw = strings.TrimSpace(fw)
		chain = strings.TrimSpace(chain)
		errs = multierror.Append(errs, c.collectStats(fw, chain, ctx))
	}

	return errs.ErrorOrNil()
}

func (c *firewallCollector) collectStats(firewall, chain string, ctx *metrics.CollectorContext) error {
	if firewall != "filter" && firewall != "mangle" && firewall != "raw" && firewall != "nat" {
		return config.InvalidConfigurationError("unknown firewall '" + firewall + "'")
	}

	if chain == "" {
		return config.InvalidConfigurationError("missing chain")
	}

	reply, err := ctx.Client.Run("/ip/firewall/"+firewall+"/print",
		"=stats=", "=chain="+chain, "?disabled=no",
		"=.proplist=comment,bytes,packets")
	if err != nil {
		return fmt.Errorf("fetch fw stats %s/%s error: %w", firewall, chain, err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		if comment := re.Map["comment"]; comment != "" {
			lctx := ctx.WithLabels(firewall, chain, comment)

			if err := c.metrics.Collect(re.Map, &lctx); err != nil {
				errs = multierror.Append(errs,
					fmt.Errorf("collect fw %s/%s error: %w", firewall, chain, err))
			}
		}
	}

	return errs.ErrorOrNil()
}
