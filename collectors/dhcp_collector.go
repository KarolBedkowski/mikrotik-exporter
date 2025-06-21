package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("dhcp", newDHCPCollector, "retrieves DHCP server metrics")
}

type dhcpCollector struct {
	leasesActiveCount RetMetric
}

func newDHCPCollector() RouterOSCollector {
	const prefix = "dhcp"

	labelNames := []string{"name", "address", "server"}

	return &dhcpCollector{
		leasesActiveCount: NewRetGaugeMetric(prefix, "leases_active", labelNames).
			WithHelp("number of active leases per DHCP server").
			Build(),
	}
}

func (c *dhcpCollector) Describe(ch chan<- *prometheus.Desc) {
	c.leasesActiveCount.Describe(ch)
}

func (c *dhcpCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/ip/dhcp-server/print", "?disabled=false", "=.proplist=name")
	if err != nil {
		return fmt.Errorf("fetch dhcp-server error: %w", err)
	}

	var errs *multierror.Error

	for _, n := range extractPropertyFromReplay(reply, "name") {
		if err := c.collectForDHCPServer(ctx, n); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}

func (c *dhcpCollector) collectForDHCPServer(ctx *CollectorContext, dhcpServer string) error {
	reply, err := ctx.client.Run("/ip/dhcp-server/lease/print", "?server="+dhcpServer, "=active=", "=count-only=")
	if err != nil {
		return fmt.Errorf("fetch lease for %s  error: %w", dhcpServer, err)
	}

	lctx := ctx.withLabels(dhcpServer)

	if err := c.leasesActiveCount.Collect(reply, &lctx); err != nil {
		return fmt.Errorf("collect active leases for %s error: %w", dhcpServer, err)
	}

	return nil
}
