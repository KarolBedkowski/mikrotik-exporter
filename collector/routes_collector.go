package collector

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("routes", newRoutesCollector)
}

type routesCollector struct {
	protocols []string

	count         retMetricCollector
	countProtocol retMetricCollector
}

func newRoutesCollector() routerOSCollector {
	const prefix = "routes"

	labelNames := []string{"name", "address", "ip_version"}

	c := &routesCollector{
		count: newRetGaugeMetric("", prefix, labelNames).
			withHelp("number of routes in RIB").build(),
		countProtocol: newRetGaugeMetric(prefix, "protocol", append(labelNames, "protocol")).
			withHelp("number of routes per protocol in RIB").build(),
		protocols: []string{"bgp", "static", "ospf", "dynamic", "connect", "rip"},
	}

	return c
}

func (c *routesCollector) describe(ch chan<- *prometheus.Desc) {
	c.count.describe(ch)
	c.countProtocol.describe(ch)
}

func (c *routesCollector) collect(ctx *collectorContext) error {
	if err := c.colllectForIPVersion("4", "ip", ctx); err != nil {
		return err
	}

	return c.colllectForIPVersion("6", "ip", ctx)
}

func (c *routesCollector) colllectForIPVersion(ipVersion, topic string, ctx *collectorContext) error {
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

func (c *routesCollector) colllectCount(ipVersion, topic string, ctx *collectorContext) error {
	reply, err := ctx.client.Run("/"+topic+"/route/print", "?disabled=false", "=count-only=")
	if err != nil {
		return fmt.Errorf("fetch route %s error: %w", topic, err)
	}

	ctx = ctx.withLabels(ipVersion)

	if err := c.count.collect(reply, ctx); err != nil {
		return fmt.Errorf("collect router %s error: %w", topic, err)
	}

	return nil
}

func (c *routesCollector) colllectCountProtcol(ipVersion, topic, protocol string, ctx *collectorContext) error {
	reply, err := ctx.client.Run("/"+topic+"/route/print", "?disabled=false", "?"+protocol, "=count-only=")
	if err != nil {
		return fmt.Errorf("fetch route %s error: %w", topic, err)
	}

	ctx = ctx.withLabels(ipVersion, protocol)

	if err := c.countProtocol.collect(reply, ctx); err != nil {
		return fmt.Errorf("collect count protocol %s/%s error: %w", topic, protocol, err)
	}

	return nil
}
