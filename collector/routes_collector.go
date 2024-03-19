package collector

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
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
		log.WithFields(log.Fields{
			"ip_version": ipVersion,
			"device":     ctx.device.Name,
			"topic":      topic,
			"error":      err,
		}).Error("error fetching routes metrics")

		return fmt.Errorf("read route error: %w", err)
	}

	ctx = ctx.withLabels(ipVersion)

	return c.count.collect(reply, ctx)
}

func (c *routesCollector) colllectCountProtcol(ipVersion, topic, protocol string, ctx *collectorContext) error {
	reply, err := ctx.client.Run("/"+topic+"/route/print", "?disabled=false", "?"+protocol, "=count-only=")
	if err != nil {
		log.WithFields(log.Fields{
			"ip_version": ipVersion,
			"protocol":   protocol,
			"device":     ctx.device.Name,
			"error":      err,
		}).Error("error fetching routes metrics")

		return fmt.Errorf("read route %s error: %w", topic, err)
	}

	ctx = ctx.withLabels(ipVersion, protocol)

	return c.countProtocol.collect(reply, ctx)
}
