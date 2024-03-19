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
	c := &poolCollector{
		usedCount: newRetGaugeMetric(prefix, "pool_used", labelNames).
			withHelp("number of used IP/prefixes in a pool").build(),
	}

	return c
}

func (c *poolCollector) describe(ch chan<- *prometheus.Desc) {
	c.usedCount.describe(ch)
}

func (c *poolCollector) collect(ctx *collectorContext) error {
	return c.collectForIPVersion("4", "ip", ctx)
}

func (c *poolCollector) collectForIPVersion(ipVersion, topic string, ctx *collectorContext) error {
	names, err := c.fetchPoolNames(ipVersion, topic, ctx)
	if err != nil {
		return err
	}

	var errs *multierror.Error

	for _, n := range names {
		if err := c.collectForPool(ipVersion, topic, n, ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}

func (c *poolCollector) fetchPoolNames(ipVersion, topic string, ctx *collectorContext) ([]string, error) {
	_ = ipVersion

	reply, err := ctx.client.Run("/"+topic+"/pool/print", "=.proplist=name")
	if err != nil {
		return nil, fmt.Errorf("get pool %s error: %w", topic, err)
	}

	return extractPropertyFromReplay(reply, "name"), nil
}

func (c *poolCollector) collectForPool(ipVersion, topic, pool string, ctx *collectorContext) error {
	reply, err := ctx.client.Run("/"+topic+"/pool/used/print", "?pool="+pool, "=count-only=")
	if err != nil {
		return fmt.Errorf("fetch used pool %s/%s error: %w", topic, pool, err)
	}

	ctx = ctx.withLabels(ipVersion, pool)

	if err := c.usedCount.collect(reply, ctx); err != nil {
		return fmt.Errorf("collect pool %s/%s error: %w", topic, pool, err)
	}

	return nil
}
