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
	metrics PropertyMetricList
}

func newWlanSTACollector() RouterOSCollector {
	const prefix = "wlan_station"

	labelNames := []string{"name", "address", "interface", "mac_address"}

	return &wlanSTACollector{
		metrics: PropertyMetricList{
			NewPropertyGaugeMetric(prefix, "signal-to-noise", labelNames).Build(),
			NewPropertyGaugeMetric(prefix, "signal-strength", labelNames).Build(),
			NewPropertyRxTxMetric(prefix, "packets", labelNames).Build(),
			NewPropertyRxTxMetric(prefix, "bytes", labelNames).Build(),
			NewPropertyRxTxMetric(prefix, "frames", labelNames).Build(),
		},
	}
}

func (c *wlanSTACollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *wlanSTACollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/interface/wireless/registration-table/print",
		"=.proplist=interface,mac-address,signal-to-noise,signal-strength,packets,bytes,frames")
	if err != nil {
		return fmt.Errorf("fetch wireless reg error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.withLabels(re.Map["interface"], re.Map["mac-address"])

		if err := c.metrics.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}
