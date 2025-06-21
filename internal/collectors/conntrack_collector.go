package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("conntrack", newConntrackCollector, "retrieves connection tracking information")
}

type conntrackCollector struct {
	totalEntries metrics.PropertyMetric
	maxEntries   metrics.PropertyMetric
}

func newConntrackCollector() RouterOSCollector {
	const prefix = "conntrack"

	return &conntrackCollector{
		totalEntries: metrics.NewPropertyGaugeMetric(prefix, "total-entries").
			WithHelp("Number of tracked connections").
			Build(),
		maxEntries: metrics.NewPropertyGaugeMetric(prefix, "max-entries").
			WithHelp("Conntrack table capacity").
			Build(),
	}
}

func (c *conntrackCollector) Describe(ch chan<- *prometheus.Desc) {
	c.totalEntries.Describe(ch)
	c.maxEntries.Describe(ch)
}

func (c *conntrackCollector) Collect(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/ip/firewall/connection/tracking/print",
		"=.proplist=total-entries,max-entries")
	if err != nil {
		return fmt.Errorf("get tracking error: %w", err)
	}

	var errs *multierror.Error

	if len(reply.Re) > 0 {
		re := reply.Re[0]

		if err := c.totalEntries.Collect(re, ctx); err != nil {
			errs = multierror.Append(errs,
				fmt.Errorf("collect total entries error: %w", err))
		}

		if err := c.maxEntries.Collect(re, ctx); err != nil {
			errs = multierror.Append(errs,
				fmt.Errorf("collect max entries error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}
