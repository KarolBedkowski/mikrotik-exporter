package collector

import (
	"fmt"
	"strings"

	"mikrotik-exporter/routeros/proto"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type netwatchCollector struct {
	propslist    string
	descriptions map[string]*prometheus.Desc
}

func newNetwatchCollector() routerOSCollector {
	c := &netwatchCollector{}
	c.init()
	return c
}

func (c *netwatchCollector) init() {
	c.propslist = strings.Join([]string{"host", "comment", "status"}, ",")
	labelNames := []string{"name", "address", "host", "comment", "status"}
	c.descriptions = make(map[string]*prometheus.Desc)
	c.descriptions["status"] = descriptionForPropertyName("netwatch", "status", labelNames)
}

func (c *netwatchCollector) describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.descriptions {
		ch <- d
	}
}

func (c *netwatchCollector) collect(ctx *collectorContext) error {
	stats, err := c.fetch(ctx)
	if err != nil {
		return err
	}

	for _, re := range stats {
		c.collectForStat(re, ctx)
	}

	return nil
}

func (c *netwatchCollector) fetch(ctx *collectorContext) ([]*proto.Sentence, error) {
	reply, err := ctx.client.Run("/tool/netwatch/print", "?disabled=false", "=.proplist="+c.propslist)
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching netwatch metrics")
		return nil, err
	}

	return reply.Re, nil
}

func (c *netwatchCollector) collectForStat(re *proto.Sentence, ctx *collectorContext) {
	host := re.Map["host"]
	comment := re.Map["comment"]
	c.collectMetricForProperty("status", host, comment, re, ctx)
}

func (c *netwatchCollector) collectMetricForProperty(property, host, comment string, re *proto.Sentence, ctx *collectorContext) {
	desc := c.descriptions[property]
	if value := re.Map[property]; value != "" {
		var upVal, downVal, unknownVal float64

		switch value {
		case "up":
			upVal = 1
		case "unknown":
			unknownVal = 1
		case "down":
			downVal = 1
		default:
			log.WithFields(log.Fields{
				"device":   ctx.device.Name,
				"host":     host,
				"property": property,
				"value":    value,
				"error":    fmt.Errorf("unexpected netwatch status value"),
			}).Error("error parsing netwatch metric value")
		}

		ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, upVal, ctx.device.Name, ctx.device.Address, host, comment, "up")
		ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, downVal, ctx.device.Name, ctx.device.Address, host, comment, "down")
		ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, unknownVal, ctx.device.Name, ctx.device.Address, host, comment, "unknown")
	}
}
