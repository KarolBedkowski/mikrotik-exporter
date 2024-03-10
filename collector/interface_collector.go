package collector

import (
	"strconv"
	"strings"

	"mikrotik-exporter/routeros/proto"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type interfaceCollector struct {
	props        []string
	propslist    string
	descriptions map[string]*prometheus.Desc
}

func newInterfaceCollector() routerOSCollector {
	c := &interfaceCollector{}
	c.init()
	return c
}

func (c *interfaceCollector) init() {
	labelsProps := []string{"name", "type", "disabled", "comment", "slave"}
	c.props = []string{"actual-mtu", "running", "rx-byte", "tx-byte", "rx-packet", "tx-packet", "rx-error", "tx-error", "rx-drop", "tx-drop", "link-downs"}
	c.propslist = strings.Join(append(labelsProps, c.props...), ",")

	labelNames := []string{"name", "address", "interface", "type", "disabled", "comment", "running", "slave"}

	c.descriptions = make(map[string]*prometheus.Desc)
	c.descriptions["actual-mtu"] = descriptionForPropertyName("interface", "actual_mtu", labelNames)
	c.descriptions["running"] = descriptionForPropertyName("interface", "running", labelNames)
	for _, p := range c.props[2:] {
		c.descriptions[p] = descriptionForPropertyName("interface", p+"_total", labelNames)
	}
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

		return nil, err
	}

	return reply.Re, nil
}

func (c *interfaceCollector) collectForStat(re *proto.Sentence, ctx *collectorContext) {
	for _, p := range c.props {
		c.collectMetricForProperty(p, re, ctx)
	}
}

func (c *interfaceCollector) collectMetricForProperty(property string, re *proto.Sentence, ctx *collectorContext) {
	desc := c.descriptions[property]
	if value := re.Map[property]; value != "" {
		var v float64
		var vtype prometheus.ValueType = prometheus.CounterValue
		var err error

		switch property {
		case "running":
			vtype = prometheus.GaugeValue
			if value == "true" {
				v = 1
			} else {
				v = 0
			}
		case "actual-mtu":
			vtype = prometheus.GaugeValue
			fallthrough
		default:
			v, err = strconv.ParseFloat(value, 64)
			if err != nil {
				log.WithFields(log.Fields{
					"device":    ctx.device.Name,
					"interface": re.Map["name"],
					"property":  property,
					"value":     value,
					"error":     err,
				}).Error("error parsing interface metric value")
				return
			}
		}

		ctx.ch <- prometheus.MustNewConstMetric(desc, vtype, v, ctx.device.Name, ctx.device.Address,
			re.Map["name"], re.Map["type"], re.Map["disabled"], re.Map["comment"], re.Map["running"], re.Map["slave"])
	}
}
