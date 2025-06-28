package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("ipv6_neighbor", newIPv6NeighborCollector, "retrieves ipv6 neighbors metrics")
}

type ipv6NeighborCollector struct {
	metrics  metrics.PropertyMetric
	statuses *prometheus.Desc
}

func newIPv6NeighborCollector() RouterOSCollector {
	const prefix = "ipv6_neighbor"

	labelNames := []string{"address", metrics.LabelInterface, "mac_address", "dynamic", "router"}

	return &ipv6NeighborCollector{
		metrics: metrics.NewPropertyGaugeMetric(prefix, "reachable", labelNames...).
			WithConverter(convert.MetricFromBool).
			Build(),
		statuses: metrics.Description(prefix, "status", "neighbor entry statuses",
			metrics.LabelDevName, metrics.LabelDevAddress, "status"),
	}
}

func (c *ipv6NeighborCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
	ch <- c.statuses
}

func (c *ipv6NeighborCollector) Collect(ctx *metrics.CollectorContext) error {
	if ctx.Device.IPv6Disabled {
		return nil
	}

	return multierror.Append(nil,
		c.collectEntries(ctx),
		c.collectStatuses(ctx),
	).ErrorOrNil()
}

func (c *ipv6NeighborCollector) collectEntries(ctx *metrics.CollectorContext) error {
	// do not load entries if not configured
	if !ctx.FeatureCfg.BoolValue("details", false) {
		return nil
	}

	// list of props must contain all values for labels and metrics
	reply, err := ctx.Client.Run("/ipv6/neighbor/print",
		"?status=reachable",
		"=.proplist=address,mac-address,interface,dynamic,dhcp,router,status")
	if err != nil {
		return fmt.Errorf("fetch ipv6 neighbor error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		// create context with labels from reply
		lctx := ctx.WithLabelsFromMap(re.Map, "address", "interface", "mac-address",
			"comment", "dhcp", "dynamic")

		// collect metrics using context
		if err := c.metrics.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}

func (c *ipv6NeighborCollector) collectStatuses(ctx *metrics.CollectorContext) error {
	var errs *multierror.Error

	for _, status := range []string{"noarp", "incomplete", "reachable", "stale", "delay", "probe", "failed"} {
		reply, err := ctx.Client.Run("/ipv6/neighbor/print", "?status="+status, "=count-only=")
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("fetch arp status %q  error: %w", status, err))

			continue
		}

		if cnt, err := convert.MetricFromString(reply.Done.Map["ret"]); err == nil {
			ctx.Ch <- prometheus.MustNewConstMetric(c.statuses, prometheus.GaugeValue, cnt,
				ctx.Device.Name, ctx.Device.Address, status)
		} else {
			errs = multierror.Append(errs, fmt.Errorf("parse ret %v error: %w", reply, err))
		}
	}

	return errs.ErrorOrNil()
}
