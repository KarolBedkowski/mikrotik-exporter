package collector

import (
	"fmt"
	"strconv"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("health", newhealthCollector)
}

type healthCollector struct {
	descriptions map[string]*prometheus.Desc
}

func newhealthCollector() routerOSCollector {
	labelNames := []string{"name", "address"}

	c := &healthCollector{
		descriptions: make(map[string]*prometheus.Desc),
	}

	c.descriptions["voltage"] = descriptionForPropertyNameHelpText(
		"health", "voltage", labelNames, "Input voltage to the RouterOS board, in volts")
	c.descriptions["temperature"] = descriptionForPropertyNameHelpText(
		"health", "temperature", labelNames, "Temperature of RouterOS board, in degrees Celsius")
	c.descriptions["cpu-temperature"] = descriptionForPropertyNameHelpText(
		"health", "cpu-temperature", labelNames, "Temperature of RouterOS CPU, in degrees Celsius")

	return c
}

func (c *healthCollector) describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.descriptions {
		ch <- d
	}
}

func (c *healthCollector) collect(ctx *collectorContext) error {
	stats, err := c.fetch(ctx)
	if err != nil {
		return err
	}

	for _, re := range stats {
		if metric, ok := re.Map["name"]; ok {
			c.collectMetricForProperty(metric, re, ctx)
		} else {
			c.collectForStat(re, ctx)
		}
	}

	return nil
}

func (c *healthCollector) fetch(ctx *collectorContext) ([]*proto.Sentence, error) {
	reply, err := ctx.client.Run("/system/health/print")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching system health metrics")

		return nil, fmt.Errorf("get health error: %w", err)
	}

	return reply.Re, nil
}

func (c *healthCollector) collectForStat(re *proto.Sentence, ctx *collectorContext) {
	c.collectMetricForProperty("voltage", re, ctx)
	c.collectMetricForProperty("temperature", re, ctx)
	c.collectMetricForProperty("cpu-temperature", re, ctx)
}

func (c *healthCollector) collectMetricForProperty(property string, re *proto.Sentence, ctx *collectorContext) {
	var (
		metricValue float64
		err         error
	)

	name := property
	value := re.Map[property]

	if value == "" {
		var ok bool
		// TODO: check
		if value, ok = re.Map["value"]; !ok {
			return
		}
	}

	metricValue, err = strconv.ParseFloat(value, 64)
	if err != nil {
		log.WithFields(log.Fields{
			"device":   ctx.device.Name,
			"property": name,
			"value":    value,
			"error":    err,
		}).Error("error parsing system health metric value")

		return
	}

	desc := c.descriptions[name]
	ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, metricValue, ctx.device.Name, ctx.device.Address)
}
