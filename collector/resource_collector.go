package collector

import (
	"fmt"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("resource", newResourceCollector)
}

type resourceCollector struct {
	metrics []propertyMetricCollector

	versionDesc *prometheus.Desc
}

func newResourceCollector() routerOSCollector {
	const prefix = "system"

	labelNames := []string{"name", "address"}

	collector := &resourceCollector{
		metrics: []propertyMetricCollector{
			newPropertyGaugeMetric(prefix, "free-memory", labelNames).build(),
			newPropertyGaugeMetric(prefix, "total-memory", labelNames).build(),
			newPropertyGaugeMetric(prefix, "cpu-load", labelNames).build(),
			newPropertyGaugeMetric(prefix, "free-hdd-space", labelNames).build(),
			newPropertyGaugeMetric(prefix, "total-hdd-space", labelNames).build(),
			newPropertyGaugeMetric(prefix, "cpu-frequency", labelNames).build(),
			newPropertyGaugeMetric(prefix, "bad-blocks", labelNames).build(),
			newPropertyCounterMetric(prefix, "uptime", labelNames).withConverter(parseDuration).build(),
			newPropertyGaugeMetric(prefix, "cpu", labelNames).build(),
		},

		versionDesc: description("system", "routeros", "Board and system version",
			[]string{"name", "address", "board_name", "version"}),
	}

	return collector
}

func (c *resourceCollector) describe(ch chan<- *prometheus.Desc) {
	for _, c := range c.metrics {
		c.describe(ch)
	}

	ch <- c.versionDesc
}

func (c *resourceCollector) collect(ctx *collectorContext) error {
	stats, err := c.fetch(ctx)
	if err != nil {
		return err
	}

	for _, re := range stats {
		c.collectForStat(re, ctx)
	}

	return nil
}

func (c *resourceCollector) fetch(ctx *collectorContext) ([]*proto.Sentence, error) {
	reply, err := ctx.client.Run("/system/resource/print",
		"=.proplist=free-memory,total-memory,cpu-load,free-hdd-space,total-hdd-space,"+
			"cpu-frequency,bad-blocks,uptime,cpu-count,board-name,version")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching system resource metrics")

		return nil, fmt.Errorf("get resource error: %w", err)
	}

	return reply.Re, nil
}

func (c *resourceCollector) collectForStat(reply *proto.Sentence, ctx *collectorContext) {
	boardname := reply.Map["board-name"]
	version := reply.Map["version"]

	ctx.ch <- prometheus.MustNewConstMetric(c.versionDesc, prometheus.GaugeValue, 1,
		ctx.device.Name, ctx.device.Address, boardname, version)

	for _, c := range c.metrics {
		_ = c.collect(reply, ctx, nil)
	}
}
