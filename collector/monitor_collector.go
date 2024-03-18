package collector

import (
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("monitor", newMonitorCollector)
}

type monitorCollector struct {
	metrics []propertyMetricCollector
}

func newMonitorCollector() routerOSCollector {
	labelNames := []string{"name", "address", "interface"}

	const prefix = "monitor"

	c := &monitorCollector{
		metrics: []propertyMetricCollector{
			newPropertyGaugeMetric(prefix, "status", labelNames).withConverter(convertFromStatus).build(),
			newPropertyGaugeMetric(prefix, "rate", labelNames).withConverter(convertFromRate).build(),
			newPropertyGaugeMetric(prefix, "full-duplex", labelNames).withConverter(convertFromBool).build(),
		},
	}

	return c
}

func (c *monitorCollector) describe(ch chan<- *prometheus.Desc) {
	for _, c := range c.metrics {
		c.describe(ch)
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
		"=.proplist=name,status,rate,full-duplex")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching ethernet monitor info")

		return fmt.Errorf("get ethernet monitor error: %w", err)
	}

	for _, e := range reply.Re {
		labels := []string{e.Map["name"]}
		for _, c := range c.metrics {
			_ = c.collect(e, ctx, labels)
		}
	}

	return nil
}

func convertFromStatus(value string) (float64, error) {
	if value == "link-ok" {
		return 1.0, nil
	}

	return 0.0, nil
}

func convertFromRate(v string) (float64, error) {
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

	return float64(value), nil
}
