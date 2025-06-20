package collectors

// TODO: check ipv6

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"mikrotik-exporter/routeros/proto"
)

func init() {
	registerCollector("pools", newPoolCollector, "retrieves IP pool metrics")
}

type poolCollector struct {
	metrics PropertyMetricList
}

func newPoolCollector() RouterOSCollector {
	const prefix = "ip_pool"

	labelNames := []string{"name", "address", "ip_version", "pool"}

	return &poolCollector{
		PropertyMetricList{
			NewPropertyGaugeMetric(prefix, "used", labelNames).
				WithHelp("number of used IP/prefixes in a pool").
				Build(),
			NewPropertyGaugeMetric(prefix, "total", labelNames).
				WithHelp("number of total IP in a pool").
				Build(),
		},
	}
}

func (c *poolCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *poolCollector) Collect(ctx *CollectorContext) error {
	errs := multierror.Append(nil, c.collectForIPv4(ctx))

	if !ctx.device.IPv6Disabled {
		errs = multierror.Append(errs, c.collectForIPv6(ctx))
	}

	return errs.ErrorOrNil()
}

func (c *poolCollector) collectForIPv4(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/ip/pool/print", "=.proplist=name,total,used")
	if err != nil {
		return fmt.Errorf("fetch ipv4 pool error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.withLabels("4", re.Map["name"])
		if err := c.metrics.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}

func (c *poolCollector) collectForIPv6(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/ipv6/pool/used/print", "=.proplist=pool")
	if err != nil {
		return fmt.Errorf("fetch used ipv6 pool error: %w", err)
	}

	counter := countByProperty(reply.Re, "pool")

	// create fake sentence to reuse metrics for ipv4
	re := proto.Sentence{Map: make(map[string]string)}

	for pool, used := range counter {
		re.Map["used"] = strconv.Itoa(used)

		lctx := ctx.withLabels("6", pool)
		if err := c.metrics[0].Collect(&re, &lctx); err != nil {
			return fmt.Errorf("collect ipv6 pool %s error: %w", pool, err)
		}
	}

	return nil
}
