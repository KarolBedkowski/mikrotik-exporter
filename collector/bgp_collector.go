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
	registerCollector("bgp", newBGPCollector)
}

type bgpCollector struct {
	props        []string
	proplist     string
	descriptions map[string]*prometheus.Desc
}

func newBGPCollector() routerOSCollector {
	const prefix = "bgp"

	labelNames := []string{"name", "address", "session", "asn"}

	collector := &bgpCollector{
		descriptions: make(map[string]*prometheus.Desc),
	}

	collector.props = []string{
		"name", "remote-as", "state", "prefix-count",
		"updates-sent", "updates-received", "withdrawn-sent", "withdrawn-received",
	}
	collector.proplist = strings.Join(collector.props, ",")
	collector.descriptions["state"] = description(prefix, "up", "BGP session is established (up = 1)", labelNames)

	for _, p := range collector.props[3:] {
		collector.descriptions[p] = descriptionForPropertyName(prefix, p, labelNames)
	}

	return collector
}

func (c *bgpCollector) describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.descriptions {
		ch <- d
	}
}

func (c *bgpCollector) collect(ctx *collectorContext) error {
	stats, err := c.fetch(ctx)
	if err != nil {
		return err
	}

	for _, re := range stats {
		c.collectForStat(re, ctx)
	}

	return nil
}

func (c *bgpCollector) fetch(ctx *collectorContext) ([]*proto.Sentence, error) {
	reply, err := ctx.client.Run("/routing/bgp/peer/print", "=.proplist="+c.proplist)
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching bgp metrics")

		return nil, fmt.Errorf("get bgp peer error: %w", err)
	}

	return reply.Re, nil
}

func (c *bgpCollector) collectForStat(re *proto.Sentence, ctx *collectorContext) {
	asn := re.Map["remote-as"]
	session := re.Map["name"]

	for _, p := range c.props[2:] {
		c.collectMetricForProperty(p, session, asn, re, ctx)
	}
}

func (c *bgpCollector) collectMetricForProperty(
	property, session, asn string, re *proto.Sentence, ctx *collectorContext,
) {
	desc := c.descriptions[property]

	propertyVal := re.Map[property]

	value, err := c.parseValueForProperty(property, propertyVal)
	if err != nil {
		log.WithFields(log.Fields{
			"device":   ctx.device.Name,
			"session":  session,
			"property": property,
			"value":    propertyVal,
			"error":    err,
		}).Error("error parsing bgp metric value")

		return
	}

	ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
		value, ctx.device.Name, ctx.device.Address, session, asn)
}

func (c *bgpCollector) parseValueForProperty(property, value string) (float64, error) {
	if property == "state" {
		if value == "established" {
			return 1, nil
		}

		return 0, nil
	}

	if value == "" {
		return 0, nil
	}

	val, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("parse value %v error: %w", value, err)
	}

	return val, nil
}
