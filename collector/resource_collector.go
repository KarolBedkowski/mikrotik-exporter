package collector

import (
	"fmt"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("resource", newResourceCollector,
		"retrieves RouterOS system resource metrics")
}

type resourceCollector struct {
	metrics propertyMetricList

	versionDesc *prometheus.Desc
}

func newResourceCollector() routerOSCollector {
	const prefix = "system"

	labelNames := []string{"name", "address"}

	return &resourceCollector{
		metrics: propertyMetricList{
			newPropertyGaugeMetric(prefix, "free-memory", labelNames).build(),
			newPropertyGaugeMetric(prefix, "total-memory", labelNames).build(),
			newPropertyGaugeMetric(prefix, "cpu-load", labelNames).build(),
			newPropertyGaugeMetric(prefix, "free-hdd-space", labelNames).build(),
			newPropertyGaugeMetric(prefix, "total-hdd-space", labelNames).build(),
			newPropertyGaugeMetric(prefix, "cpu-frequency", labelNames).build(),
			newPropertyGaugeMetric(prefix, "bad-blocks", labelNames).build(),
			newPropertyCounterMetric(prefix, "uptime", labelNames).
				withName("uptime_seconds").withConverter(metricFromDuration).build(),
			newPropertyGaugeMetric(prefix, "cpu-count", labelNames).build(),
		},

		versionDesc: description("system", "routeros", "Board and system version",
			[]string{"name", "address", "board_name", "version"}),
	}
}

func (c *resourceCollector) describe(ch chan<- *prometheus.Desc) {
	c.metrics.describe(ch)
	ch <- c.versionDesc
}

func (c *resourceCollector) collect(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/system/resource/print",
		"=.proplist=free-memory,total-memory,cpu-load,free-hdd-space,total-hdd-space,"+
			"cpu-frequency,bad-blocks,uptime,cpu-count,board-name,version")
	if err != nil {
		return fmt.Errorf("fetch resource error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		if err := c.collectForStat(re, ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}

func (c *resourceCollector) collectForStat(reply *proto.Sentence, ctx *collectorContext) error {
	boardname := reply.Map["board-name"]
	version := reply.Map["version"]

	ctx.ch <- prometheus.MustNewConstMetric(c.versionDesc, prometheus.GaugeValue, 1,
		ctx.device.Name, ctx.device.Address, boardname, version)

	if err := c.metrics.collect(reply, ctx); err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	return nil
}
