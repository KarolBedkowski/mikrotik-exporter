package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("dhcpv6", newDHCPv6Collector, "retrieves DHCPv6 server metrics")
}

type dhcpv6Collector struct {
	bindingCount RetMetric
}

func newDHCPv6Collector() RouterOSCollector {
	const prefix = "dhcpv6"

	labelNames := []string{"name", "address", "server"}

	c := &dhcpv6Collector{
		bindingCount: NewRetGaugeMetric(prefix, "binding", labelNames).
			WithHelp("number of active bindings per DHCPv6 server").Build(),
	}

	return c
}

func (c *dhcpv6Collector) Describe(ch chan<- *prometheus.Desc) {
	c.bindingCount.Describe(ch)
}

func (c *dhcpv6Collector) Collect(ctx *CollectorContext) error {
	if ctx.device.IPv6Disabled {
		return nil
	}

	reply, err := ctx.client.Run("/ipv6/dhcp-server/print", "=.proplist=name")
	if err != nil {
		return fmt.Errorf("fetch dhcp6 server names error: %w", err)
	}

	names := extractPropertyFromReplay(reply, "name")

	var errs *multierror.Error

	for _, n := range names {
		if err := c.collectForDHCPServer(ctx, n); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}

func (c *dhcpv6Collector) collectForDHCPServer(ctx *CollectorContext, dhcpServer string) error {
	reply, err := ctx.client.Run("/ipv6/dhcp-server/binding/print",
		"?server="+dhcpServer, "=count-only=")
	if err != nil {
		return fmt.Errorf("get dhcpv6 bindings error: %w", err)
	}

	lctx := ctx.withLabels(dhcpServer)

	if err := c.bindingCount.Collect(reply, &lctx); err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	return nil
}
