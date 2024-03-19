package collector

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/prometheus/client_golang/prometheus"
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
		if err := c.collectForStat(re, ctx); err != nil {
			return err
		}
	}

	return nil
}

func (c *bgpCollector) fetch(ctx *collectorContext) ([]*proto.Sentence, error) {
	reply, err := ctx.client.Run("/routing/bgp/peer/print", "=.proplist="+c.proplist)
	if err != nil {
		return nil, fmt.Errorf("fetch bgp peer error: %w", err)
	}

	return reply.Re, nil
}

func (c *bgpCollector) collectForStat(re *proto.Sentence, ctx *collectorContext) error {
	asn := re.Map["remote-as"]
	session := re.Map["name"]

	for _, p := range c.props[2:] {
		if err := c.collectMetricForProperty(p, session, asn, re, ctx); err != nil {
			return fmt.Errorf("collect %s error: %w", p, err)
		}
	}

	return nil
}

func (c *bgpCollector) collectMetricForProperty(
	property, session, asn string, re *proto.Sentence, ctx *collectorContext,
) error {
	desc := c.descriptions[property]
	propertyVal := re.Map[property]

	value, err := c.parseValueForProperty(property, propertyVal)
	if err != nil {
		return fmt.Errorf("parse %v error: %w", propertyVal, err)
	}

	ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
		value, ctx.device.Name, ctx.device.Address, session, asn)

	return nil
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
