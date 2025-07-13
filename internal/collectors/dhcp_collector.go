package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("dhcp", newDHCPCollector, "retrieves DHCP server metrics")
}

type dhcpCollector struct {
	leasesActiveCount metrics.PropertyMetric
}

func newDHCPCollector() RouterOSCollector {
	const prefix = "dhcp"

	return &dhcpCollector{
		leasesActiveCount: metrics.NewPropertyRetMetric(prefix, "leases_active", "server").
			WithHelp("number of active leases per DHCP server").
			WithConverter(convert.MetricFromString).
			Build(),
	}
}

func (c *dhcpCollector) Describe(ch chan<- *prometheus.Desc) {
	c.leasesActiveCount.Describe(ch)
}

func (c *dhcpCollector) Collect(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/ip/dhcp-server/print", "?disabled=false", "=.proplist=name")
	if err != nil {
		return fmt.Errorf("fetch dhcp-server error: %w", err)
	}

	var errs *multierror.Error

	for _, n := range convert.ExtractPropertyFromReplay(reply, "name") {
		if err := c.collectForDHCPServer(ctx, n); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}

func (c *dhcpCollector) collectForDHCPServer(ctx *metrics.CollectorContext, dhcpServer string) error {
	reply, err := ctx.Client.Run("/ip/dhcp-server/lease/print", "?server="+dhcpServer, "=active=", "=count-only=")
	if err != nil {
		return fmt.Errorf("fetch lease for %s  error: %w", dhcpServer, err)
	}

	lctx := ctx.WithLabels(dhcpServer)

	if err := c.leasesActiveCount.Collect(reply.Done.Map, &lctx); err != nil {
		return fmt.Errorf("collect active leases for %s error: %w", dhcpServer, err)
	}

	return nil
}
