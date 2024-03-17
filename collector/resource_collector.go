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
	versionDesc       *prometheus.Desc
	freeMemoryDesc    *prometheus.Desc
	totalMemoryDesc   *prometheus.Desc
	cpuLoadDesc       *prometheus.Desc
	freeHddSpaceDesc  *prometheus.Desc
	totalHddSpaceDesc *prometheus.Desc
	cpuFrequencyDesc  *prometheus.Desc
	badBlocksDesc     *prometheus.Desc
	uptimeDesc        *prometheus.Desc
	cpuCountDesc      *prometheus.Desc
}

func newResourceCollector() routerOSCollector {
	labelNames := []string{"name", "address"}

	collector := &resourceCollector{
		freeMemoryDesc:    descriptionForPropertyName("system", "free-memory", labelNames),
		totalMemoryDesc:   descriptionForPropertyName("system", "total-memory", labelNames),
		cpuLoadDesc:       descriptionForPropertyName("system", "cpu-load", labelNames),
		freeHddSpaceDesc:  descriptionForPropertyName("system", "free-hdd-space", labelNames),
		totalHddSpaceDesc: descriptionForPropertyName("system", "total-hdd-space", labelNames),
		cpuFrequencyDesc:  descriptionForPropertyName("system", "cpu-frequency", labelNames),
		badBlocksDesc:     descriptionForPropertyName("system", "bad-blocks", labelNames),
		uptimeDesc:        descriptionForPropertyName("system", "uptime_total", labelNames),
		cpuCountDesc:      descriptionForPropertyName("system", "cput", labelNames),
		versionDesc: description("system", "routeros", "Board and system version",
			[]string{"name", "address", "board_name", "version"}),
	}

	return collector
}

func (c *resourceCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.freeMemoryDesc
	ch <- c.totalMemoryDesc
	ch <- c.cpuLoadDesc
	ch <- c.freeHddSpaceDesc
	ch <- c.totalHddSpaceDesc
	ch <- c.cpuFrequencyDesc
	ch <- c.badBlocksDesc
	ch <- c.uptimeDesc
	ch <- c.cpuCountDesc
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

	pcl := newPropertyCollector(reply, ctx)
	_ = pcl.collectCounterValue(c.uptimeDesc, "uptime", parseDuration)
	_ = pcl.collectGaugeValue(c.freeMemoryDesc, "free-memory", nil)
	_ = pcl.collectGaugeValue(c.totalMemoryDesc, "total-memory", nil)
	_ = pcl.collectGaugeValue(c.cpuLoadDesc, "cpu-load", nil)
	_ = pcl.collectGaugeValue(c.freeHddSpaceDesc, "free-hdd-space", nil)
	_ = pcl.collectGaugeValue(c.totalHddSpaceDesc, "total-hdd-space", nil)
	_ = pcl.collectGaugeValue(c.cpuFrequencyDesc, "cpu-frequency", nil)
	_ = pcl.collectGaugeValue(c.badBlocksDesc, "bad-blocks", nil)
	_ = pcl.collectGaugeValue(c.cpuCountDesc, "cput", nil)
}
