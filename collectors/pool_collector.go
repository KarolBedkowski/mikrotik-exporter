package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("pools", newPoolCollector, "retrieves IP/IPv6 pool metrics")
}

type poolCollector struct {
	usedCount RetMetric
}

func newPoolCollector() RouterOSCollector {
	const prefix = "ip_pool"

	labelNames := []string{"name", "address", "ip_version", "pool"}

	return &poolCollector{
		usedCount: NewRetGaugeMetric(prefix, "pool_used", labelNames).
			WithHelp("number of used IP/prefixes in a pool").Build(),
	}
}

func (c *poolCollector) Describe(ch chan<- *prometheus.Desc) {
	c.usedCount.Describe(ch)
}

func (c *poolCollector) Collect(ctx *CollectorContext) error {
	errs := multierror.Append(nil, c.colllectForIPVersion("4", "ip", ctx))

	if !ctx.device.IPv6Disabled {
		errs = multierror.Append(errs, c.colllectForIPVersion("6", "ipv6", ctx))
	}

	return errs.ErrorOrNil()
}

func (c *poolCollector) colllectForIPVersion(ipVersion, topic string, ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/"+topic+"/pool/print", "=.proplist=name")
	if err != nil {
		return fmt.Errorf("fetch %s pool error: %w", topic, err)
	}

	pools := extractPropertyFromReplay(reply, "name")

	var errs *multierror.Error

	for _, n := range pools {
		if err := c.collectForPool(ipVersion, topic, n, ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}

func (c *poolCollector) collectForPool(ipVersion, topic, pool string, ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/"+topic+"/pool/used/print", "?pool="+pool, "=count-only=")
	if err != nil {
		return fmt.Errorf("fetch used ip pool %s error: %w", pool, err)
	}

	lctx := ctx.withLabels(ipVersion, pool)

	if err := c.usedCount.Collect(reply, &lctx); err != nil {
		return fmt.Errorf("collect ip pool %s error: %w", pool, err)
	}

	return nil
}
