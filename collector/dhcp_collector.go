package collector

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
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
	if err != nil || len(names) == 0 {
		return err
	}

	var errs *multierror.Error

	for _, n := range names {
		if err := c.colllectForDHCPServer(ctx, n); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}

func (c *dhcpCollector) fetchDHCPServerNames(ctx *collectorContext) ([]string, error) {
	reply, err := ctx.client.Run("/ip/dhcp-server/print", "=.proplist=name")
	if err != nil {
		return nil, fmt.Errorf("fetch dhcp-server error: %w", err)
	}

	return extractPropertyFromReplay(reply, "name"), nil
}

func (c *dhcpCollector) colllectForDHCPServer(ctx *collectorContext, dhcpServer string) error {
	reply, err := ctx.client.Run("/ip/dhcp-server/lease/print", "?server="+dhcpServer, "=active=", "=count-only=")
	if err != nil {
		return fmt.Errorf("fetch lease for %s  error: %w", dhcpServer, err)
	}

	ctx = ctx.withLabels(dhcpServer)

	if err := c.leasesActiveCount.collect(reply, ctx); err != nil {
		return fmt.Errorf("collect active leases error: %w", err)
	}

	return nil
}
