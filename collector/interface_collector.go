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
	registerCollector("interface", newInterfaceCollector)
}

type interfaceCollector struct {
	props        []string
	propslist    string
	descriptions map[string]*prometheus.Desc
}

func newInterfaceCollector() routerOSCollector {
	labelsProps := []string{"name", "type", "disabled", "comment", "slave"}
	labelNames := []string{"name", "address", "interface", "type", "disabled", "comment", "running", "slave"}

	collector := &interfaceCollector{
		descriptions: make(map[string]*prometheus.Desc),
	}

	collector.props = []string{
		"actual-mtu", "running", "rx-byte", "tx-byte", "rx-packet", "tx-packet",
		"rx-error", "tx-error", "rx-drop", "tx-drop", "link-downs",
	}
	collector.propslist = strings.Join(append(labelsProps, collector.props...), ",")
	collector.descriptions["actual-mtu"] = descriptionForPropertyName("interface", "actual_mtu", labelNames)
	collector.descriptions["running"] = descriptionForPropertyName("interface", "running", labelNames)

	for _, p := range collector.props[2:] {
		collector.descriptions[p] = descriptionForPropertyName("interface", p+"_total", labelNames)
	}

	return collector
}

func (c *interfaceCollector) describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.descriptions {
		ch <- d
	}
}

func (c *interfaceCollector) collect(ctx *collectorContext) error {
	stats, err := c.fetch(ctx)
	if err != nil {
		return err
	}

	for _, re := range stats {
		c.collectForStat(re, ctx)
	}

	return nil
}

func (c *interfaceCollector) fetch(ctx *collectorContext) ([]*proto.Sentence, error) {
	reply, err := ctx.client.Run("/interface/print", "=.proplist="+c.propslist)
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching interface metrics")

		return nil, fmt.Errorf("get interfaces detail error: %w", err)
	}

	return reply.Re, nil
}

func (c *interfaceCollector) collectForStat(re *proto.Sentence, ctx *collectorContext) {
	for _, p := range c.props {
		c.collectMetricForProperty(p, re, ctx)
	}
}

func (c *interfaceCollector) collectMetricForProperty(
	property string, reply *proto.Sentence, ctx *collectorContext,
) {
	desc := c.descriptions[property]

	if value := reply.Map[property]; value != "" {
		var (
			metricValue float64
			vtype       = prometheus.CounterValue
			err         error
		)

		switch property {
		case "running":
			vtype = prometheus.GaugeValue
			metricValue = parseBool(value)
		case "actual-mtu":
			vtype = prometheus.GaugeValue

			fallthrough
		default:
			metricValue, err = strconv.ParseFloat(value, 64)
			if err != nil {
				log.WithFields(log.Fields{
					"device":    ctx.device.Name,
					"interface": reply.Map["name"],
					"property":  property,
					"value":     value,
					"error":     err,
				}).Error("error parsing interface metric value")

				return
			}
		}

		ctx.ch <- prometheus.MustNewConstMetric(desc, vtype, metricValue, ctx.device.Name, ctx.device.Address,
			reply.Map["name"], reply.Map["type"], reply.Map["disabled"], reply.Map["comment"],
			reply.Map["running"], reply.Map["slave"])
	}
}
