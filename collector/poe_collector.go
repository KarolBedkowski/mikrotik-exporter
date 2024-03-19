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
	current propertyMetricCollector
	power   propertyMetricCollector
	voltage propertyMetricCollector
}

func newPOECollector() routerOSCollector {
	const prefix = "poe"

	labelNames := []string{"name", "address", "interface"}

	return &poeCollector{
		current: newPropertyGaugeMetric(prefix, "current", labelNames).
			withHelp("current in mA").build(),
		power: newPropertyGaugeMetric(prefix, "wattage", labelNames).
			withHelp("power in W").build(),
		voltage: newPropertyGaugeMetric(prefix, "voltage", labelNames).
			withHelp("voltage in V").build(),
	}
}

func (c *poeCollector) describe(ch chan<- *prometheus.Desc) {
	c.current.describe(ch)
	c.power.describe(ch)
	c.voltage.describe(ch)
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
			_ = c.current.collect(se, ctx, []string{name})
			_ = c.voltage.collect(se, ctx, []string{name})
			_ = c.power.collect(se, ctx, []string{name})
		}
	}

	return nil
}
