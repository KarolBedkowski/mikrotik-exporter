package collector

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("dhcp", newDHCPCollector)
}

type dhcpCollector struct {
	leasesActiveCount retMetricCollector
}

func newDHCPCollector() routerOSCollector {
	const prefix = "dhcp"

	labelNames := []string{"name", "address", "server"}

	c := &dhcpCollector{
		leasesActiveCount: newRetGaugeMetric(prefix, "leases_active", labelNames).
			withHelp("number of active leases per DHCP server").build(),
	}

	return c
}

func (c *dhcpCollector) describe(ch chan<- *prometheus.Desc) {
	c.leasesActiveCount.describe(ch)
}

func (c *dhcpCollector) collect(ctx *collectorContext) error {
	names, err := c.fetchDHCPServerNames(ctx)
	if err != nil {
		return err
	}

	for _, n := range names {
		if err := c.colllectForDHCPServer(ctx, n); err != nil {
			return err
		}
	}

	return nil
}

func (c *dhcpCollector) fetchDHCPServerNames(ctx *collectorContext) ([]string, error) {
	reply, err := ctx.client.Run("/ip/dhcp-server/print", "=.proplist=name")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching DHCP server names")

		return nil, fmt.Errorf("get dhcp-server error: %w", err)
	}

	names := []string{}
	for _, re := range reply.Re {
		names = append(names, re.Map["name"])
	}

	return names, nil
}

func (c *dhcpCollector) colllectForDHCPServer(ctx *collectorContext, dhcpServer string) error {
	reply, err := ctx.client.Run("/ip/dhcp-server/lease/print", "?server="+dhcpServer, "=active=", "=count-only=")
	if err != nil {
		log.WithFields(log.Fields{
			"dhcp_server": dhcpServer,
			"device":      ctx.device.Name,
			"error":       err,
		}).Error("error fetching DHCP lease counts")

		return fmt.Errorf("get lease error: %w", err)
	}

	ctx = ctx.withLabels(dhcpServer)

	return c.leasesActiveCount.collect(reply, ctx)
}
