package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/metrics"
	"mikrotik-exporter/routeros/proto"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("dhcpl", newDHCPLCollector, "retrieves DHCP server lease information")
}

type dhcpLeaseCollector struct {
	// leases is one metric per lease; enabled by "details: true".
	leases metrics.PropertyMetric
	// statuses report number of leases by status.
	statuses *prometheus.Desc
}

func newDHCPLCollector() RouterOSCollector {
	const prefix = "dhcp"

	labelNames := []string{
		"activemacaddress", "server", "status", "activeaddress",
		"hostname", metrics.LabelComment, "dhcp_address", "dhcp_macaddress",
	}

	return &dhcpLeaseCollector{
		leases: metrics.NewPropertyConstMetric(prefix, "status", labelNames...).
			WithName("leases_status").
			Build(),
		statuses: metrics.Description(prefix, "leases_by_status", "Number of DHCP leases by status",
			metrics.LabelDevName, metrics.LabelDevAddress, "status"),
	}
}

func (c *dhcpLeaseCollector) Describe(ch chan<- *prometheus.Desc) {
	c.leases.Describe(ch)
	ch <- c.statuses
}

func (c *dhcpLeaseCollector) Collect(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/ip/dhcp-server/lease/print",
		"?disabled=false",
		"=.proplist=active-mac-address,server,status,active-address,host-name,comment,address,mac-address")
	if err != nil {
		return fmt.Errorf("fetch dhcp lease error: %w", err)
	}

	// Count statuses
	for status, count := range metrics.CountByProperty(reply.Re, "status") {
		ctx.Ch <- prometheus.MustNewConstMetric(c.statuses, prometheus.GaugeValue, float64(count),
			ctx.Device.Name, ctx.Device.Address, status)
	}

	// do not load entries if not configured
	if !ctx.FeatureCfg.BoolValue("details", false) {
		return nil
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		if err := c.collectMetric(ctx, re); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}

func (c *dhcpLeaseCollector) collectMetric(ctx *metrics.CollectorContext, re *proto.Sentence) error {
	lctx := ctx.WithLabels(
		re.Map["active-mac-address"], re.Map["server"], re.Map["status"],
		re.Map["active-address"], metrics.CleanHostName(re.Map["host-name"]),
		re.Map["comment"], re.Map["address"], re.Map["mac-address"],
	)

	if err := c.leases.Collect(re.Map, &lctx); err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	return nil
}
