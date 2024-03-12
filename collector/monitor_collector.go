package collector

import (
	"fmt"
	"strings"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("monitor", newMonitorCollector)
}

type monitorCollector struct {
	props        []string // props from monitor, can add other ether props later if needed
	propslist    string
	descriptions map[string]*prometheus.Desc
}

func newMonitorCollector() routerOSCollector {
	c := &monitorCollector{
		descriptions: make(map[string]*prometheus.Desc),
		props:        []string{"status", "rate", "full-duplex"},
	}
	c.propslist = strings.Join(c.props, ",")

	labelNames := []string{"name", "address", "interface"}
	for _, p := range c.props {
		c.descriptions[p] = descriptionForPropertyName("monitor", p, labelNames)
	}

	return c
}

func (c *monitorCollector) describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.descriptions {
		ch <- d
	}
}

func (c *monitorCollector) collect(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/interface/ethernet/print", "=.proplist=name")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching ethernet interfaces")

		return fmt.Errorf("get ethernet error: %w", err)
	}

	eths := make([]string, len(reply.Re))
	for idx, eth := range reply.Re {
		eths[idx] = eth.Map["name"]
	}

	return c.collectForMonitor(eths, ctx)
}

func (c *monitorCollector) collectForMonitor(eths []string, ctx *collectorContext) error {
	reply, err := ctx.client.Run("/interface/ethernet/monitor",
		"=numbers="+strings.Join(eths, ","),
		"=once=",
		"=.proplist=name,"+c.propslist)
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching ethernet monitor info")

		return fmt.Errorf("get ethernet monitor error: %w", err)
	}

	for _, e := range reply.Re {
		c.collectMetricsForEth(e.Map["name"], e, ctx)
	}

	return nil
}

func (c *monitorCollector) collectMetricsForEth(name string, se *proto.Sentence, ctx *collectorContext) {
	for _, prop := range c.props {
		v, ok := se.Map[prop]
		if !ok {
			continue
		}

		value := float64(c.valueForProp(prop, v))
		ctx.ch <- prometheus.MustNewConstMetric(c.descriptions[prop], prometheus.GaugeValue,
			value, ctx.device.Name, ctx.device.Address, name)
	}
}

func (c *monitorCollector) valueForProp(name, value string) int {
	val := 0

	switch name {
	case "status":
		if value == "link-ok" {
			val = 1
		}

	case "rate":
		switch value {
		case "10Mbps":
			val = 10
		case "100Mbps":
			val = 100
		case "1Gbps":
			val = 1000
		case "2.5Gbps":
			val = 2500
		case "5Gbps":
			val = 5000
		case "10Gbps":
			val = 10000
		case "25Gbps":
			val = 25000
		case "40Gbps":
			val = 40000
		case "50Gbps":
			val = 50000
		case "100Gbps":
			val = 100000
		}

	case "full-duplex":
		if value == "true" {
			val = 1
		}
	}

	return val
}
