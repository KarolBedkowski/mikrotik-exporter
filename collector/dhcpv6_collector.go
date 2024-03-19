package collector

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("dhcpv6", newDHCPv6Collector)
}

type dhcpv6Collector struct {
	bindingCount retMetricCollector
}

func newDHCPv6Collector() routerOSCollector {
	const prefix = "dhcpv6"

	labelNames := []string{"name", "address", "server"}

	c := &dhcpv6Collector{
		bindingCount: newRetGaugeMetric(prefix, "binding", labelNames).
			withHelp("number of active bindings per DHCPv6 server").build(),
	}

	return c
}

func (c *dhcpv6Collector) describe(ch chan<- *prometheus.Desc) {
	c.bindingCount.describe(ch)
}

func (c *dhcpv6Collector) collect(ctx *collectorContext) error {
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

func (c *dhcpv6Collector) fetchDHCPServerNames(ctx *collectorContext) ([]string, error) {
	reply, err := ctx.client.Run("/ipv6/dhcp-server/print", "=.proplist=name")
	if err != nil {
		return nil, fmt.Errorf("fetch dhcp6 server names error: %w", err)
	}

	return extractPropertyFromReplay(reply, "name"), nil
}

func (c *dhcpv6Collector) colllectForDHCPServer(ctx *collectorContext, dhcpServer string) error {
	reply, err := ctx.client.Run("/ipv6/dhcp-server/binding/print", "?server="+dhcpServer, "=count-only=")
	if err != nil {
		return fmt.Errorf("get dhcpv6 bindings error: %w", err)
	}

	ctx = ctx.withLabels(dhcpServer)

	if err := c.bindingCount.collect(reply, ctx); err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	return nil
}
