package collectors

import (
	"fmt"
	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/internal/metrics"
	"mikrotik-exporter/routeros/proto"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("resource", newResourceCollector,
		"retrieves RouterOS system resource metrics")
}

type resourceCollector struct {
	versionDesc *prometheus.Desc
	metrics     metrics.PropertyMetricList
}

func newResourceCollector() RouterOSCollector {
	const prefix = "system"

	return &resourceCollector{
		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyGaugeMetric(prefix, "free-memory").Build(),
			metrics.NewPropertyGaugeMetric(prefix, "total-memory").Build(),
			metrics.NewPropertyGaugeMetric(prefix, "cpu-load").Build(),
			metrics.NewPropertyGaugeMetric(prefix, "free-hdd-space").Build(),
			metrics.NewPropertyGaugeMetric(prefix, "total-hdd-space").Build(),
			metrics.NewPropertyGaugeMetric(prefix, "cpu-frequency").Build(),
			metrics.NewPropertyGaugeMetric(prefix, "bad-blocks").Build(),
			metrics.NewPropertyCounterMetric(prefix, "uptime").
				WithName("uptime_seconds").
				WithConverter(convert.MetricFromDuration).
				Build(),
			metrics.NewPropertyGaugeMetric(prefix, "cpu-count").Build(),
		},

		versionDesc: metrics.Description("system", "routeros", "Board and system version",
			metrics.LabelDevName, metrics.LabelDevAddress, "board_name", "version"),
	}
}

func (c *resourceCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
	ch <- c.versionDesc
}

func (c *resourceCollector) Collect(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/system/resource/print",
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

func (c *resourceCollector) collectForStat(reply *proto.Sentence, ctx *metrics.CollectorContext) error {
	boardname := reply.Map["board-name"]
	version := reply.Map["version"]

	ctx.Ch <- prometheus.MustNewConstMetric(c.versionDesc, prometheus.GaugeValue, 1,
		ctx.Device.Name, ctx.Device.Address, boardname, version)

	if err := c.metrics.Collect(reply, ctx); err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	return nil
}
