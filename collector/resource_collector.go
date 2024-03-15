package collector

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("resource", newResourceCollector)
}

type resourceCollector struct {
	props        []string
	descriptions map[string]*prometheus.Desc
	versionDesc  *prometheus.Desc
}

func newResourceCollector() routerOSCollector {
	collector := &resourceCollector{
		descriptions: make(map[string]*prometheus.Desc),
	}

	collector.props = []string{
		"free-memory", "total-memory", "cpu-load", "free-hdd-space", "total-hdd-space", "cpu-frequency", "bad-blocks",
		"uptime", "cpu-count",
		"board-name", "version",
	}

	labelNames := []string{"name", "address"}

	for _, p := range collector.props[:len(collector.props)-4] {
		collector.descriptions[p] = descriptionForPropertyName("system", p, labelNames)
	}

	collector.descriptions["cpu-count"] = descriptionForPropertyName("system", "cpu", labelNames)
	collector.descriptions["uptime"] = descriptionForPropertyName("system", "uptime_total", labelNames)
	collector.versionDesc = description("system", "routeros", "Board and system version",
		[]string{"name", "address", "board_name", "version"})

	return collector
}

func (c *resourceCollector) describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.descriptions {
		ch <- d
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
	reply, err := ctx.client.Run("/system/resource/print", "=.proplist="+strings.Join(c.props, ","))
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching system resource metrics")

		return nil, fmt.Errorf("get resource error: %w", err)
	}

	return reply.Re, nil
}

func (c *resourceCollector) collectForStat(re *proto.Sentence, ctx *collectorContext) {
	boardname := re.Map["board-name"]
	version := re.Map["version"]

	ctx.ch <- prometheus.MustNewConstMetric(c.versionDesc, prometheus.GaugeValue, 1,
		ctx.device.Name, ctx.device.Address, boardname, version)

	for _, p := range c.props[:9] {
		c.collectMetricForProperty(p, re, ctx)
	}
}

func (c *resourceCollector) collectMetricForProperty(property string, reply *proto.Sentence, ctx *collectorContext) {
	value := reply.Map[property]
	if value == "" {
		return
	}

	var (
		metricValue float64
		vtype       = prometheus.GaugeValue
		err         error
	)

	if property == "uptime" {
		metricValue, err = parseDuration(value)
		vtype = prometheus.CounterValue
	} else {
		metricValue, err = strconv.ParseFloat(value, 64)
	}

	if err != nil {
		log.WithFields(log.Fields{
			"device":   ctx.device.Name,
			"property": property,
			"value":    reply.Map[property],
			"error":    err,
		}).Error("error parsing system resource metric value")

		return
	}

	desc := c.descriptions[property]
	ctx.ch <- prometheus.MustNewConstMetric(desc, vtype, metricValue, ctx.device.Name, ctx.device.Address)
}
