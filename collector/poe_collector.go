package collector

import (
	"strconv"
	"strings"

	"mikrotik-exporter/routeros/proto"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("poe", newPOECollector)
}

type poeCollector struct {
	currentDesc *prometheus.Desc
	powerDesc   *prometheus.Desc
	voltageDesc *prometheus.Desc
	props       []string
	propslist   string
}

func newPOECollector() routerOSCollector {
	const prefix = "poe"

	labelNames := []string{"name", "address", "interface"}
	props := []string{"poe-out-current", "poe-out-voltage", "poe-out-power"}
	return &poeCollector{
		currentDesc: description(prefix, "current", "current in mA", labelNames),
		powerDesc:   description(prefix, "wattage", "Power in W", labelNames),
		voltageDesc: description(prefix, "voltage", "Voltage in V", labelNames),
		props:       props,
		propslist:   strings.Join(append(props, "name"), ","),
	}
}

func (c *poeCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.currentDesc
	ch <- c.powerDesc
	ch <- c.voltageDesc
}

func (c *poeCollector) collect(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/interface/ethernet/poe/print", "=.proplist=name")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching interface poe metrics")

		return err
	}

	ifaces := make([]string, 0)
	for _, iface := range reply.Re {
		n := iface.Map["name"]
		ifaces = append(ifaces, n)
	}

	if len(ifaces) == 0 {
		return nil
	}

	return c.collectPOEMetricsForInterfaces(ifaces, ctx)
}

func (c *poeCollector) collectPOEMetricsForInterfaces(ifaces []string, ctx *collectorContext) error {
	reply, err := ctx.client.Run("/interface/ethernet/poe/monitor", "=numbers="+strings.Join(ifaces, ","), "=once=", "=.proplist="+c.propslist)
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching interface poe monitor metrics")

		return err
	}

	for _, se := range reply.Re {
		if name, ok := se.Map["name"]; ok {
			c.collectMetricsForInterface(name, se, ctx)
		}
	}

	return nil
}

func (c *poeCollector) collectMetricsForInterface(name string, se *proto.Sentence, ctx *collectorContext) {
	for _, prop := range c.props {
		v, ok := se.Map[prop]
		if !ok || v == "" {
			continue
		}

		value, err := strconv.ParseFloat(v, 64)
		if err != nil {
			log.WithFields(log.Fields{
				"device":    ctx.device.Name,
				"interface": name,
				"property":  prop,
				"error":     err,
			}).Error("error parsing interface poe monitor metric")

			return
		}

		ctx.ch <- prometheus.MustNewConstMetric(c.descForKey(prop), prometheus.GaugeValue, value, ctx.device.Name, ctx.device.Address, name)
	}
}

func (c *poeCollector) descForKey(name string) *prometheus.Desc {
	switch name {
	case "poe-out-current":
		return c.currentDesc
	case "poe-out-voltage":
		return c.voltageDesc
	case "poe-out-power":
		return c.powerDesc
	}

	return nil
}
