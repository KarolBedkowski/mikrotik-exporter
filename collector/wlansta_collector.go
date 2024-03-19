package collector

import (
	"fmt"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("wlansta", newWlanSTACollector)
}

type wlanSTACollector struct {
	metrics []propertyMetricCollector
}

func newWlanSTACollector() routerOSCollector {
	const prefix = "wlan_station"

	labelNames := []string{"name", "address", "interface", "mac_address"}

	collector := &wlanSTACollector{
		metrics: []propertyMetricCollector{
			newPropertyGaugeMetric(prefix, "signal-to-noise", labelNames).build(),
			newPropertyGaugeMetric(prefix, "signal-strength", labelNames).build(),
			newPropertyRxTxMetric(prefix, "packets", labelNames).build(),
			newPropertyRxTxMetric(prefix, "bytes", labelNames).build(),
			newPropertyRxTxMetric(prefix, "frames", labelNames).build(),
		},
	}

	return collector
}

func (c *wlanSTACollector) describe(ch chan<- *prometheus.Desc) {
	for _, c := range c.metrics {
		c.describe(ch)
	}
}

func (c *wlanSTACollector) collect(ctx *collectorContext) error {
	stats, err := c.fetch(ctx)
	if err != nil {
		return err
	}

	for _, re := range stats {
		ctx = ctx.withLabels(re.Map["interface"], re.Map["mac-address"])

		for _, c := range c.metrics {
			_ = c.collect(re, ctx)
		}
	}

	return nil
}

func (c *wlanSTACollector) fetch(ctx *collectorContext) ([]*proto.Sentence, error) {
	reply, err := ctx.client.Run("/interface/wireless/registration-table/print",
		"=.proplist=interface,mac-address,signal-to-noise,signal-strength,packets,bytes,frames")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching wlan station metrics")

		return nil, fmt.Errorf("read wireless reg error: %w", err)
	}

	return reply.Re, nil
}
