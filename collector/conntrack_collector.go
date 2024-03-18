package collector

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("conntrack", newConntrackCollector)
}

type conntrackCollector struct {
	totalEntries propertyMetricCollector
	maxEntries   propertyMetricCollector
}

func newConntrackCollector() routerOSCollector {
	const prefix = "conntrack"

	labelNames := []string{"name", "address"}

	return &conntrackCollector{
		totalEntries: newPropertyGaugeMetric(prefix, "total-entries", labelNames).
			withHelp("Number of tracked connections").build(),
		maxEntries: newPropertyGaugeMetric(prefix, "max-entries", labelNames).
			withHelp("Conntrack table capacity").build(),
	}
}

func (c *conntrackCollector) describe(ch chan<- *prometheus.Desc) {
	c.totalEntries.describe(ch)
	c.maxEntries.describe(ch)
}

func (c *conntrackCollector) collect(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/ip/firewall/connection/tracking/print",
		"=.proplist=total-entries,max-entries")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching conntrack table metrics")

		return fmt.Errorf("get tracking error: %w", err)
	}

	if len(reply.Re) > 0 {
		re := reply.Re[0]
		_ = c.totalEntries.collect(re, ctx, nil)
		_ = c.maxEntries.collect(re, ctx, nil)
	}

	return nil
}
