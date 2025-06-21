package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("routes", newRoutesCollector,
		"retrieves routing table information")
}

type routesCollector struct {
	count         metrics.RetMetric
	countProtocol metrics.RetMetric
	protocols     []string
}

func newRoutesCollector() RouterOSCollector {
	const prefix = "routes"

	return &routesCollector{
		count: metrics.NewRetGaugeMetric("", prefix, "ip_version").
			WithHelp("number of routes in RIB").
			Build(),
		countProtocol: metrics.NewRetGaugeMetric(prefix, "protocol", "ip_version", "protocol").
			WithHelp("number of routes per protocol in RIB").
			Build(),
		protocols: []string{"bgp", "static", "ospf", "dynamic", "connect", "rip"},
	}
}

func (c *routesCollector) Describe(ch chan<- *prometheus.Desc) {
	c.count.Describe(ch)
	c.countProtocol.Describe(ch)
}

func (c *routesCollector) Collect(ctx *metrics.CollectorContext) error {
	errs := multierror.Append(nil, c.collectForIPVersion("4", "ip", ctx))

	if !ctx.Device.IPv6Disabled {
		errs = multierror.Append(errs, c.collectForIPVersion("6", "ipv6", ctx))
	}

	return errs.ErrorOrNil()
}

func (c *routesCollector) collectForIPVersion(ipVersion, topic string, ctx *metrics.CollectorContext) error {
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

func (c *routesCollector) collectCount(ipVersion, topic string, ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/"+topic+"/route/print", "?disabled=false", "=count-only=")
	if err != nil {
		return fmt.Errorf("fetch route %s error: %w", topic, err)
	}

	lctx := ctx.WithLabels(ipVersion)

	if err := c.count.Collect(reply, &lctx); err != nil {
		return fmt.Errorf("collect router %s %s error: %w", topic, ipVersion, err)
	}

	return nil
}

func (c *routesCollector) collectCountProtocol(ipVersion, topic, protocol string, ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/"+topic+"/route/print", "?disabled=false", "?"+protocol, "=count-only=")
	if err != nil {
		return fmt.Errorf("fetch route %s error: %w", topic, err)
	}

	lctx := ctx.WithLabels(ipVersion, protocol)

	if err := c.countProtocol.Collect(reply, &lctx); err != nil {
		return fmt.Errorf("collect count protocol %s %s error: %w", topic, protocol, err)
	}

	return nil
}
