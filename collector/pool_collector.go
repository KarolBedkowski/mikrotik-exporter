package collector

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("pools", newPoolCollector)
}

type poolCollector struct {
	usedCount retMetricCollector
}

func newPoolCollector() routerOSCollector {
	const prefix = "ip_pool"

	labelNames := []string{"name", "address", "ip_version", "pool"}

	return &poolCollector{
		usedCount: newRetGaugeMetric(prefix, "pool_used", labelNames).
			withHelp("number of used IP/prefixes in a pool").build(),
	}
}

func (c *poolCollector) describe(ch chan<- *prometheus.Desc) {
	c.usedCount.describe(ch)
}

func (c *poolCollector) collect(ctx *collectorContext) error {
	return multierror.Append(nil,
		c.collectTopic("4", "ip", ctx),
		c.collectTopic("6", "ipv6", ctx),
	).ErrorOrNil()
}

func (c *poolCollector) collectTopic(ipVersion, topic string, ctx *collectorContext) error {
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

func (c *poolCollector) collectForPool(ipVersion, topic, pool string, ctx *collectorContext) error {
	reply, err := ctx.client.Run("/"+topic+"/pool/used/print", "?pool="+pool, "=count-only=")
	if err != nil {
		return fmt.Errorf("fetch used ip pool %s error: %w", pool, err)
	}

	ctx = ctx.withLabels(ipVersion, pool)

	if err := c.usedCount.collect(reply, ctx); err != nil {
		return fmt.Errorf("collect ip pool %s error: %w", pool, err)
	}

	return nil
}
