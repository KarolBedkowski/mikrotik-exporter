package collector

import (
	"fmt"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("health", newhealthCollector)
}

type healthCollector struct {
	metrics []propertyMetricCollector
}

func newhealthCollector() routerOSCollector {
	const prefix = "health"

	labelNames := []string{"name", "address"}

	c := &healthCollector{
		metrics: []propertyMetricCollector{
			newPropertyGaugeMetric(prefix, "voltage", labelNames).
				withHelp("Input voltage to the RouterOS board, in volts").build(),
			newPropertyGaugeMetric(prefix, "temperature", labelNames).
				withHelp("Temperature of RouterOS board, in degrees Celsius").build(),
			newPropertyGaugeMetric(prefix, "cpu-temperature", labelNames).
				withHelp("Temperature of RouterOS CPU, in degrees Celsius").build(),
		},
	}

	return c
}

func (c *healthCollector) describe(ch chan<- *prometheus.Desc) {
	for _, m := range c.metrics {
		m.describe(ch)
	}
}

func (c *healthCollector) collect(ctx *collectorContext) error {
	stats, err := c.fetch(ctx)
	if err != nil {
		return err
	}

	for _, re := range stats {
		if metric, ok := re.Map["name"]; ok {
			if v, ok := re.Map["value"]; ok {
				re.Map[metric] = v
			} else {
				continue
			}
		}

		for _, c := range c.metrics {
			_ = c.collect(re, ctx, nil)
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
