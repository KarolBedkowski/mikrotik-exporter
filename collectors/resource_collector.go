package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"mikrotik-exporter/routeros/proto"
)

func init() {
	registerCollector("resource", newResourceCollector,
		"retrieves RouterOS system resource metrics")
}

type resourceCollector struct {
	metrics PropertyMetricList

	versionDesc *prometheus.Desc
}

func newResourceCollector() RouterOSCollector {
	const prefix = "system"

	labelNames := []string{"name", "address"}

	return &resourceCollector{
		metrics: PropertyMetricList{
			NewPropertyGaugeMetric(prefix, "free-memory", labelNames).Build(),
			NewPropertyGaugeMetric(prefix, "total-memory", labelNames).Build(),
			NewPropertyGaugeMetric(prefix, "cpu-load", labelNames).Build(),
			NewPropertyGaugeMetric(prefix, "free-hdd-space", labelNames).Build(),
			NewPropertyGaugeMetric(prefix, "total-hdd-space", labelNames).Build(),
			NewPropertyGaugeMetric(prefix, "cpu-frequency", labelNames).Build(),
			NewPropertyGaugeMetric(prefix, "bad-blocks", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "uptime", labelNames).
				WithName("uptime_seconds").WithConverter(metricFromDuration).Build(),
			NewPropertyGaugeMetric(prefix, "cpu-count", labelNames).Build(),
		},

		versionDesc: description("system", "routeros", "Board and system version",
			[]string{"name", "address", "board_name", "version"}),
	}
}

func (c *resourceCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
	ch <- c.versionDesc
}

func (c *resourceCollector) Collect(ctx *CollectorContext) error {
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

func (c *resourceCollector) collectForStat(reply *proto.Sentence, ctx *CollectorContext) error {
	boardname := reply.Map["board-name"]
	version := reply.Map["version"]

	ctx.ch <- prometheus.MustNewConstMetric(c.versionDesc, prometheus.GaugeValue, 1,
		ctx.device.Name, ctx.device.Address, boardname, version)

	if err := c.metrics.Collect(reply, ctx); err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	return nil
}
