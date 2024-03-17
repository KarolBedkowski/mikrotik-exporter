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
	protocols         []string
	countDesc         *prometheus.Desc
	countProtocolDesc *prometheus.Desc
}

func newRoutesCollector() routerOSCollector {
	const prefix = "routes"

	labelNames := []string{"name", "address", "ip_version"}

	c := &routesCollector{}
	c.countDesc = description("", prefix, "number of routes in RIB", labelNames)
	c.countProtocolDesc = description(prefix, "protocol", "number of routes per protocol in RIB",
		append(labelNames, "protocol"))
	c.protocols = []string{"bgp", "static", "ospf", "dynamic", "connect", "rip"}

	return c
}

func (c *routesCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.countDesc
	ch <- c.countProtocolDesc
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

	rcl := newRetCollector(reply, ctx, ipVersion)

	return rcl.collectGaugeValue(c.countDesc, nil)
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

	rcl := newRetCollector(reply, ctx, ipVersion, protocol)

	return rcl.collectGaugeValue(c.countProtocolDesc, nil)
}
