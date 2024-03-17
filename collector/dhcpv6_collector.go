package collector

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("dhcpv6", newDHCPv6Collector)
}

type dhcpv6Collector struct {
	bindingCountDesc *prometheus.Desc
}

func newDHCPv6Collector() routerOSCollector {
	const prefix = "dhcpv6"

	labelNames := []string{"name", "address", "server"}

	c := &dhcpv6Collector{
		bindingCountDesc: description(prefix, "binding", "number of active bindings per DHCPv6 server", labelNames),
	}

	return c
}

func (c *dhcpv6Collector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.bindingCountDesc
}

func (c *dhcpv6Collector) collect(ctx *collectorContext) error {
	names, err := c.fetchDHCPServerNames(ctx)
	if err != nil {
		return err
	}

	for _, n := range names {
		err := c.colllectForDHCPServer(ctx, n)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *dhcpv6Collector) fetchDHCPServerNames(ctx *collectorContext) ([]string, error) {
	reply, err := ctx.client.Run("/ipv6/dhcp-server/print", "=.proplist=name")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching DHCPv6 server names")

		return nil, fmt.Errorf("get dhcp-server error: %w", err)
	}

	names := []string{}
	for _, re := range reply.Re {
		names = append(names, re.Map["name"])
	}

	return names, nil
}

func (c *dhcpv6Collector) colllectForDHCPServer(ctx *collectorContext, dhcpServer string) error {
	reply, err := ctx.client.Run("/ipv6/dhcp-server/binding/print", "?server="+dhcpServer, "=count-only=")
	if err != nil {
		log.WithFields(log.Fields{
			"dhcpv6_server": dhcpServer,
			"device":        ctx.device.Name,
			"error":         err,
		}).Error("error fetching DHCPv6 binding counts")

		return fmt.Errorf("get bindings error: %w", err)
	}

	rcl := newRetCollector(reply, ctx, dhcpServer)
	_ = rcl.collectGaugeValue(c.bindingCountDesc, nil)

	return nil
}
