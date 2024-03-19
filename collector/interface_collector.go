package collector

import (
	"fmt"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("interface", newInterfaceCollector)
}

type interfaceCollector struct {
	metrics propertyMetricList
}

func newInterfaceCollector() routerOSCollector {
	const prefix = "interface"

	labelNames := []string{"name", "address", "interface", "type", "disabled", "comment", "running", "slave"}

	collector := &interfaceCollector{
		metrics: propertyMetricList{
			newPropertyGaugeMetric(prefix, "actual-mtu", labelNames).build(),
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
	c.metrics.describe(ch)
}

func (c *interfaceCollector) collect(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/interface/print",
		"=.proplist=name,type,disabled,comment,slave,actual-mtu,running,rx-byte,tx-byte,"+
			"rx-packet,tx-packet,rx-error,tx-error,rx-drop,tx-drop,link-downs")
	if err != nil {
		return fmt.Errorf("fetch interfaces detail error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		if err := c.collectForStat(re, ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}

func (c *interfaceCollector) collectForStat(reply *proto.Sentence, ctx *collectorContext) error {
	ctx = ctx.withLabels(
		reply.Map["name"], reply.Map["type"], reply.Map["disabled"], reply.Map["comment"],
		reply.Map["running"], reply.Map["slave"],
	)

	return c.metrics.collect(reply, ctx)
}
