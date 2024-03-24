package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("routes", newRoutesCollector,
		"retrieves routing table information")
}

type routesCollector struct {
	protocols []string

	count         retMetricCollector
	countProtocol retMetricCollector
}

func newRoutesCollector() RouterOSCollector {
	const prefix = "routes"

	labelNames := []string{"name", "address", "ip_version"}

	return &routesCollector{
		count: newRetGaugeMetric("", prefix, labelNames).
			withHelp("number of routes in RIB").build(),
		countProtocol: newRetGaugeMetric(prefix, "protocol", append(labelNames, "protocol")).
			withHelp("number of routes per protocol in RIB").build(),
		protocols: []string{"bgp", "static", "ospf", "dynamic", "connect", "rip"},
	}
}

func (c *routesCollector) Describe(ch chan<- *prometheus.Desc) {
	c.count.describe(ch)
	c.countProtocol.describe(ch)
}

func (c *routesCollector) Collect(ctx *CollectorContext) error {
	errs := multierror.Append(nil, c.colllectForIPVersion("4", "ip", ctx))

	if !ctx.device.IPv6Disabled {
		errs = multierror.Append(errs, c.colllectForIPVersion("6", "ipv6", ctx))
	}

	return errs.ErrorOrNil()
}

func (c *routesCollector) colllectForIPVersion(ipVersion, topic string, ctx *CollectorContext) error {
	if err := c.colllectCount(ipVersion, topic, ctx); err != nil {
		return err
	}

	for _, p := range c.protocols {
		if err := c.colllectCountProtcol(ipVersion, topic, p, ctx); err != nil {
			return err
		}
	}

	return nil
}

func (c *routesCollector) colllectCount(ipVersion, topic string, ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/"+topic+"/route/print", "?disabled=false", "=count-only=")
	if err != nil {
		return fmt.Errorf("fetch route %s error: %w", topic, err)
	}

	ctx = ctx.withLabels(ipVersion)

	if err := c.count.collect(reply, ctx); err != nil {
		return fmt.Errorf("collect router %s %s error: %w", topic, ipVersion, err)
	}

	return nil
}

func (c *routesCollector) colllectCountProtcol(ipVersion, topic, protocol string, ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/"+topic+"/route/print", "?disabled=false", "?"+protocol, "=count-only=")
	if err != nil {
		return fmt.Errorf("fetch route %s error: %w", topic, err)
	}

	ctx = ctx.withLabels(ipVersion, protocol)

	if err := c.countProtocol.collect(reply, ctx); err != nil {
		return fmt.Errorf("collect count protocol %s %s error: %w", topic, protocol, err)
	}

	return nil
}
