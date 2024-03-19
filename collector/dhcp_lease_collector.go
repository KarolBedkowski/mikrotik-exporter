package collector

import (
	"fmt"
	"strconv"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("dhcpl", newDHCPLCollector)
}

type dhcpLeaseCollector struct {
	leases propertyMetricCollector
}

func newDHCPLCollector() routerOSCollector {
	labelNames := []string{
		"name", "address", "activemacaddress", "server", "status", "activeaddress",
		"hostname", "comment",
	}
	c := &dhcpLeaseCollector{
		leases: newPropertyGaugeMetric("dhcp", "status", labelNames).
			withName("leases_metrics").withHelp("number of metrics").
			withConverter(convertToOne).build(),
	}

	return c
}

func (c *dhcpLeaseCollector) describe(ch chan<- *prometheus.Desc) {
	c.leases.describe(ch)
}

func (c *dhcpLeaseCollector) collect(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/ip/dhcp-server/lease/print", "?status=bound",
		"=.proplist=active-mac-address,server,status,active-address,host-name,comment")
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

func (c *dhcpLeaseCollector) collectMetric(ctx *collectorContext, re *proto.Sentence) error {
	hostname := re.Map["host-name"]
	if hostname != "" {
		if hostname[0] == '"' {
			hostname = hostname[1 : len(hostname)-1]
		}

		// QuoteToASCII because of broken DHCP clients
		hostname = strconv.QuoteToASCII(hostname)
		hostname = hostname[1 : len(hostname)-1]
	}

	ctx = ctx.withLabels(
		re.Map["active-mac-address"], re.Map["server"], re.Map["status"],
		re.Map["active-address"], hostname, re.Map["comment"],
	)

	if err := c.leases.collect(re, ctx); err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	return nil
}
