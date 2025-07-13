package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("neighbor", newNeighborCollector,
		"retrieves neighbor metrics")
}

type neighborCollector struct {
	metrics metrics.PropertyMetricList
	stats   *prometheus.Desc
}

func newNeighborCollector() RouterOSCollector {
	const prefix = "neighbor"

	// values for first two labels (dev_name and dev_address) are added automatically;
	// rest must be loaded in Collect.
	labelNames := []string{
		"about", "address4", "discovered-by", metrics.LabelInterface, "ipv6", "platform", "software-id", "version",
		"neighbor-address", "address6", "board", "identity", "interface-name", "mac-address", "system-caps",
		"system-description",
	}

	return &neighborCollector{
		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyConstMetric(prefix, "address", labelNames...).WithName("entry").Build(),
		},
		stats: metrics.Description(prefix, "discovered", "Number of discovered neighbors by interface",
			metrics.LabelDevName, metrics.LabelDevAddress, "interface"),
	}
}

func (c *neighborCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *neighborCollector) Collect(ctx *metrics.CollectorContext) error {
	proplist := "=.proplist=address4,discovered-by,interface,ipv6,platform,software-id,version,address," +
		"address6,board,identity,interface-name,mac-address,system-caps,system-description"

	// RO6 not know some properties so return none...
	if ctx.Device.FirmwareVersion.Major < 7 { //nolint:mnd
		proplist = "=.proplist=address4,interface,ipv6,platform,software-id,version,address," +
			"address6,board,identity,interface-name,mac-address,system-caps,system-description"
	}

	reply, err := ctx.Client.Run("/ip/neighbor/print", proplist)
	if err != nil {
		return fmt.Errorf("fetch neighbor error: %w", err)
	}

	// Count statuses for complete entries; failed and incomplete must be counted separately.
	for iface, count := range metrics.CountByProperty(reply.Re, "interface") {
		ctx.Ch <- prometheus.MustNewConstMetric(c.stats, prometheus.GaugeValue, float64(count),
			ctx.Device.Name, ctx.Device.Address, iface)
	}

	// do not load entries if not configured
	if !ctx.FeatureCfg.BoolValue("details", false) {
		return nil
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.WithLabelsFromMap(re.Map,
			"about", "address4", "discovered-by", "interface", "ipv6", "platform", "software-id", "version",
			"address", "address6", "board", "identity", "interface-name", "mac-address", "system-caps",
			"system-description",
		)

		if err := c.metrics.Collect(re.Map, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}
