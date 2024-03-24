package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("wlansta", newWlanSTACollector,
		"retrieves connecten WLAN station metrics")
}

type wlanSTACollector struct {
	metrics propertyMetricList
}

func newWlanSTACollector() RouterOSCollector {
	const prefix = "wlan_station"

	labelNames := []string{"name", "address", "interface", "mac_address"}

	return &wlanSTACollector{
		metrics: propertyMetricList{
			newPropertyGaugeMetric(prefix, "signal-to-noise", labelNames).build(),
			newPropertyGaugeMetric(prefix, "signal-strength", labelNames).build(),
			newPropertyRxTxMetric(prefix, "packets", labelNames).build(),
			newPropertyRxTxMetric(prefix, "bytes", labelNames).build(),
			newPropertyRxTxMetric(prefix, "frames", labelNames).build(),
		},
	}
}

func (c *wlanSTACollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.describe(ch)
}

func (c *wlanSTACollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/interface/wireless/registration-table/print",
		"=.proplist=interface,mac-address,signal-to-noise,signal-strength,packets,bytes,frames")
	if err != nil {
		return fmt.Errorf("fetch wireless reg error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		ctx = ctx.withLabels(re.Map["interface"], re.Map["mac-address"])

		if err := c.metrics.collect(re, ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}
