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
	count         RetMetric
	countProtocol RetMetric
	protocols     []string
}

func newRoutesCollector() RouterOSCollector {
	const prefix = "routes"

	labelNames := []string{"name", "address", "ip_version"}

	return &routesCollector{
		count: NewRetGaugeMetric("", prefix, labelNames).
			WithHelp("number of routes in RIB").Build(),
		countProtocol: NewRetGaugeMetric(prefix, "protocol", append(labelNames, "protocol")).
			WithHelp("number of routes per protocol in RIB").Build(),
		protocols: []string{"bgp", "static", "ospf", "dynamic", "connect", "rip"},
	}
}

func (c *routesCollector) Describe(ch chan<- *prometheus.Desc) {
	c.count.Describe(ch)
	c.countProtocol.Describe(ch)
}

func (c *routesCollector) Collect(ctx *CollectorContext) error {
	errs := multierror.Append(nil, c.collectForIPVersion("4", "ip", ctx))

	if !ctx.device.IPv6Disabled {
		errs = multierror.Append(errs, c.collectForIPVersion("6", "ipv6", ctx))
	}

	return errs.ErrorOrNil()
}

func (c *routesCollector) collectForIPVersion(ipVersion, topic string, ctx *CollectorContext) error {
	if err := c.collectCount(ipVersion, topic, ctx); err != nil {
		return err
	}

	for _, p := range c.protocols {
		if err := c.collectCountProtocol(ipVersion, topic, p, ctx); err != nil {
			return err
		}
	}

	return nil
}

func (c *routesCollector) collectCount(ipVersion, topic string, ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/"+topic+"/route/print", "?disabled=false", "=count-only=")
	if err != nil {
		return fmt.Errorf("fetch route %s error: %w", topic, err)
	}

	lctx := ctx.withLabels(ipVersion)

	if err := c.count.Collect(reply, &lctx); err != nil {
		return fmt.Errorf("collect router %s %s error: %w", topic, ipVersion, err)
	}

	return nil
}

func (c *routesCollector) collectCountProtocol(ipVersion, topic, protocol string, ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/"+topic+"/route/print", "?disabled=false", "?"+protocol, "=count-only=")
	if err != nil {
		return fmt.Errorf("fetch route %s error: %w", topic, err)
	}

	lctx := ctx.withLabels(ipVersion, protocol)

	if err := c.countProtocol.Collect(reply, &lctx); err != nil {
		return fmt.Errorf("collect count protocol %s %s error: %w", topic, protocol, err)
	}

	return nil
}
