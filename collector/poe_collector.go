package collector

import (
	"fmt"
	"strings"

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
}

func newPOECollector() routerOSCollector {
	const prefix = "poe"

	labelNames := []string{"name", "address", "interface"}

	return &poeCollector{
		currentDesc: description(prefix, "current", "current in mA", labelNames),
		powerDesc:   description(prefix, "wattage", "Power in W", labelNames),
		voltageDesc: description(prefix, "voltage", "Voltage in V", labelNames),
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

		return fmt.Errorf("get ethernet poe error: %w", err)
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
	reply, err := ctx.client.Run("/interface/ethernet/poe/monitor",
		"=numbers="+strings.Join(ifaces, ","), "=once=",
		"=.proplist=poe-out-current,poe-out-voltage,poe-out-power")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching interface poe monitor metrics")

		return fmt.Errorf("get poe monitor error: %w", err)
	}

	for _, se := range reply.Re {
		if name, ok := se.Map["name"]; ok {
			pcl := newPropertyCollector(se, ctx, name)
			_ = pcl.collectGaugeValue(c.currentDesc, "poe-out-current", nil)
			_ = pcl.collectGaugeValue(c.voltageDesc, "poe-out-voltage", nil)
			_ = pcl.collectGaugeValue(c.powerDesc, "poe-out-power", nil)
		}
	}

	return nil
}
