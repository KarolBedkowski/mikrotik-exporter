package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("dhcpv6", newDHCPv6Collector, "retrieves DHCPv6 server metrics")
}

type dhcpv6Collector struct {
	bindingCount metrics.RetMetric
}

func newDHCPv6Collector() RouterOSCollector {
	const prefix = "dhcpv6"

	return &dhcpv6Collector{
		bindingCount: metrics.NewRetGaugeMetric(prefix, "binding", "server").
			WithHelp("number of active bindings per DHCPv6 server").
			Build(),
	}
}

func (c *dhcpv6Collector) Describe(ch chan<- *prometheus.Desc) {
	c.bindingCount.Describe(ch)
}

func (c *dhcpv6Collector) Collect(ctx *metrics.CollectorContext) error {
	if ctx.Device.IPv6Disabled {
		return nil
	}

	reply, err := ctx.Client.Run("/ipv6/dhcp-server/print", "?disabled=false", "=.proplist=name")
	if err != nil {
		return fmt.Errorf("fetch dhcp6 server names error: %w", err)
	}

	var errs *multierror.Error

	for _, n := range metrics.ExtractPropertyFromReplay(reply, "name") {
		if err := c.collectForDHCPServer(ctx, n); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}

func (c *dhcpv6Collector) collectForDHCPServer(ctx *metrics.CollectorContext, dhcpServer string) error {
	reply, err := ctx.Client.Run("/ipv6/dhcp-server/binding/print",
		"?server="+dhcpServer, "=count-only=")
	if err != nil {
		return fmt.Errorf("get dhcpv6 bindings error: %w", err)
	}

	lctx := ctx.WithLabels(dhcpServer)

	if err := c.bindingCount.Collect(reply, &lctx); err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	return nil
}
