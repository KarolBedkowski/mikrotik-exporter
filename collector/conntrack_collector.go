package collector

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("conntrack", newConntrackCollector,
		"retrieves connection tracking information")
}

type conntrackCollector struct {
	totalEntries propertyMetricCollector
	maxEntries   propertyMetricCollector
}

func newConntrackCollector() routerOSCollector {
	const prefix = "conntrack"

	labelNames := []string{"name", "address"}

	return &conntrackCollector{
		totalEntries: newPropertyGaugeMetric(prefix, "total-entries", labelNames).
			withHelp("Number of tracked connections").build(),
		maxEntries: newPropertyGaugeMetric(prefix, "max-entries", labelNames).
			withHelp("Conntrack table capacity").build(),
	}
}

func (c *conntrackCollector) describe(ch chan<- *prometheus.Desc) {
	c.totalEntries.describe(ch)
	c.maxEntries.describe(ch)
}

func (c *conntrackCollector) collect(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/ip/firewall/connection/tracking/print",
		"=.proplist=total-entries,max-entries")
	if err != nil {
		return fmt.Errorf("get tracking error: %w", err)
	}

	var errs *multierror.Error

	if len(reply.Re) > 0 {
		re := reply.Re[0]

		if err := c.totalEntries.collect(re, ctx); err != nil {
			errs = multierror.Append(errs,
				fmt.Errorf("collect total entries error: %w", err))
		}

		if err := c.maxEntries.collect(re, ctx); err != nil {
			errs = multierror.Append(errs,
				fmt.Errorf("collect max entries error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}
