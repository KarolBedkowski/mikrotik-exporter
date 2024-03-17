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
	totalEntriesDesc *prometheus.Desc
	maxEntriesDesc   *prometheus.Desc
}

func newConntrackCollector() routerOSCollector {
	const prefix = "conntrack"

	labelNames := []string{"name", "address"}

	return &conntrackCollector{
		totalEntriesDesc: description(prefix, "entries", "Number of tracked connections", labelNames),
		maxEntriesDesc:   description(prefix, "max_entries", "Conntrack table capacity", labelNames),
	}
}

func (c *conntrackCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.totalEntriesDesc
	ch <- c.maxEntriesDesc
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
		pcl := newPropertyCollector(re, ctx)
		_ = pcl.collectGaugeValue(c.totalEntriesDesc, "total-entries", nil)
		_ = pcl.collectGaugeValue(c.maxEntriesDesc, "max-entries", nil)
	}

	return nil
}
