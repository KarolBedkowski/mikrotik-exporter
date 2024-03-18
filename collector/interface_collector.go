package collector

import (
	"fmt"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("interface", newInterfaceCollector)
}

type interfaceCollector struct {
	metrics []propertyMetricCollector
}

func newInterfaceCollector() routerOSCollector {
	const prefix = "interface"

	labelNames := []string{"name", "address", "interface", "type", "disabled", "comment", "running", "slave"}

	collector := &interfaceCollector{
		metrics: []propertyMetricCollector{
			newPropertyGaugeMetric(prefix, "actual_mtu", labelNames).build(),
			newPropertyGaugeMetric(prefix, "running", labelNames).withConverter(convertFromBool).build(),
			newPropertyCounterMetric(prefix, "rx-byte", labelNames).build(),
			newPropertyCounterMetric(prefix, "tx-byte", labelNames).build(),
			newPropertyCounterMetric(prefix, "rx-packet", labelNames).build(),
			newPropertyCounterMetric(prefix, "tx-packet", labelNames).build(),
			newPropertyCounterMetric(prefix, "rx-error", labelNames).build(),
			newPropertyCounterMetric(prefix, "tx-error", labelNames).build(),
			newPropertyCounterMetric(prefix, "rx-drop", labelNames).build(),
			newPropertyCounterMetric(prefix, "tx-drop", labelNames).build(),
			newPropertyCounterMetric(prefix, "link-downs", labelNames).build(),
		},
	}

	return collector
}

func (c *interfaceCollector) describe(ch chan<- *prometheus.Desc) {
	for _, p := range c.metrics {
		p.describe(ch)
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
	reply, err := ctx.client.Run("/interface/print",
		"=.proplist=name,type,disabled,comment,slave,actual-mtu,running,rx-byte,tx-byte,"+
			"rx-packet,tx-packet,rx-error,tx-error,rx-drop,tx-drop,link-downs")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching interface metrics")

		return nil, fmt.Errorf("get interfaces detail error: %w", err)
	}

	return reply.Re, nil
}

func (c *interfaceCollector) collectForStat(reply *proto.Sentence, ctx *collectorContext) {
	labels := []string{
		reply.Map["name"], reply.Map["type"], reply.Map["disabled"], reply.Map["comment"],
		reply.Map["running"], reply.Map["slave"],
	}

	for _, p := range c.metrics {
		_ = p.collect(reply, ctx, labels)
	}
}
