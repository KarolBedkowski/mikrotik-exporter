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
	propslist       string
	ifaceStatusDesc *prometheus.Desc
	ifaceRateDesc   *prometheus.Desc
	ifaceDuplexDesc *prometheus.Desc
}

func newMonitorCollector() routerOSCollector {
	labelNames := []string{"name", "address", "interface"}

	c := &monitorCollector{
		propslist:       "status,rate,full-duplex",
		ifaceStatusDesc: descriptionForPropertyName("monitor", "status", labelNames),
		ifaceRateDesc:   descriptionForPropertyName("monitor", "rate", labelNames),
		ifaceDuplexDesc: descriptionForPropertyName("monitor", "full-duplex", labelNames),
	}

	return c
}

func (c *monitorCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.ifaceStatusDesc
	ch <- c.ifaceRateDesc
	ch <- c.ifaceDuplexDesc
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
		name := e.Map["name"]
		c.collectStatus(name, e, ctx)
		c.collectRate(name, e, ctx)
		c.collectDuplex(name, e, ctx)
	}

	return nil
}

func (c *monitorCollector) collectStatus(name string, se *proto.Sentence, ctx *collectorContext) {
	v, ok := se.Map["status"]
	if !ok {
		return
	}

	value := 0.0

	if v == "link-ok" {
		value = 1.0
	}

	ctx.ch <- prometheus.MustNewConstMetric(c.ifaceStatusDesc, prometheus.GaugeValue,
		value, ctx.device.Name, ctx.device.Address, name)
}

func (c *monitorCollector) collectRate(name string, se *proto.Sentence, ctx *collectorContext) {
	v, ok := se.Map["rate"]
	if !ok {
		return
	}

	value := 0

	switch v {
	case "10Mbps":
		value = 10
	case "100Mbps":
		value = 100
	case "1Gbps":
		value = 1000
	case "2.5Gbps":
		value = 2500
	case "5Gbps":
		value = 5000
	case "10Gbps":
		value = 10000
	case "25Gbps":
		value = 25000
	case "40Gbps":
		value = 40000
	case "50Gbps":
		value = 50000
	case "100Gbps":
		value = 100000
	}

	ctx.ch <- prometheus.MustNewConstMetric(c.ifaceRateDesc, prometheus.GaugeValue,
		float64(value), ctx.device.Name, ctx.device.Address, name)
}

func (c *monitorCollector) collectDuplex(name string, se *proto.Sentence, ctx *collectorContext) {
	v, ok := se.Map["full-duplex"]
	if !ok {
		return
	}

	value := 0

	if v == "true" {
		value = 1.0
	}

	ctx.ch <- prometheus.MustNewConstMetric(c.ifaceDuplexDesc, prometheus.GaugeValue,
		float64(value), ctx.device.Name, ctx.device.Address, name)
}
