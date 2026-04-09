package collectors

import (
	"errors"
	"fmt"

	"mikrotik-exporter/internal/metrics"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("wlansta", newWlanSTACollector,
		"retrieves connecten WLAN station metrics")
}

type wlanSTACollector struct {
	metrics metrics.PropertyMetricList
}

func newWlanSTACollector() RouterOSCollector {
	const prefix = "wlan_station"

	labelNames := []string{metrics.LabelInterface, "mac_address"}

	return &wlanSTACollector{
		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyGaugeMetric(prefix, "signal-to-noise", labelNames...).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "signal-strength", labelNames...).Build(),
			metrics.NewPropertyRxTxMetric(prefix, "packets", labelNames...).Build(),
			metrics.NewPropertyRxTxMetric(prefix, "bytes", labelNames...).Build(),
			metrics.NewPropertyRxTxMetric(prefix, "frames", labelNames...).Build(),
		},
	}
}

func (c *wlanSTACollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *wlanSTACollector) Collect(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/interface/wireless/registration-table/print",
		"=.proplist=interface,mac-address,signal-to-noise,signal-strength,packets,bytes,frames")
	if err != nil {
		return fmt.Errorf("fetch wireless reg error: %w", err)
	}

	var errs error

	for _, re := range reply.Re {
		lctx := ctx.WithLabelsFromMap(re.Map, "interface", "mac-address")

		if err := c.metrics.Collect(re.Map, &lctx); err != nil {
			errs = errors.Join(errs, err)
		}
	}

	return errs
}
