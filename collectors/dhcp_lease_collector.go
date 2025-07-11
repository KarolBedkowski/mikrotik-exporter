package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"mikrotik-exporter/routeros/proto"
)

func init() {
	registerCollector("dhcpl", newDHCPLCollector, "retrieves DHCP server lease information")
}

type dhcpLeaseCollector struct {
	leases PropertyMetric
}

func newDHCPLCollector() RouterOSCollector {
	labelNames := []string{
		"name", "address", "activemacaddress", "server", "status", "activeaddress",
		"hostname", "comment", "dhcp_address", "dhcp_macaddress",
	}

	return &dhcpLeaseCollector{
		leases: NewPropertyConstMetric("dhcp", "status", labelNames).
			WithName("leases_metrics").
			WithHelp("number of metrics").
			Build(),
	}
}

func (c *dhcpLeaseCollector) Describe(ch chan<- *prometheus.Desc) {
	c.leases.Describe(ch)
}

func (c *dhcpLeaseCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/ip/dhcp-server/lease/print",
		"?disabled=false",
		"=.proplist=active-mac-address,server,status,active-address,host-name,comment,address,mac-address")
	if err != nil {
		return fmt.Errorf("fetch dhcp lease error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		if err := c.collectMetric(ctx, re); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}

func (c *dhcpLeaseCollector) collectMetric(ctx *CollectorContext, re *proto.Sentence) error {
	lctx := ctx.withLabels(
		re.Map["active-mac-address"], re.Map["server"], re.Map["status"],
		re.Map["active-address"], cleanHostName(re.Map["host-name"]),
		re.Map["comment"], re.Map["address"], re.Map["mac-address"],
	)

	if err := c.leases.Collect(re, &lctx); err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	return nil
}
