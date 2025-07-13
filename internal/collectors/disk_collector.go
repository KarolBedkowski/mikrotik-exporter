package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("disk", newDiskCollector, "retrieves disks metrics")
}

type diskCollector struct {
	metrics metrics.PropertyMetricList
	entries metrics.PropertyMetric
}

func newDiskCollector() RouterOSCollector {
	const prefix = "disk"

	labelNames := []string{"slot", "fs-uuid", "mount-point"}
	entryLabelNames := []string{"slot", "type", "fs-uuid", metrics.LabelComment, "parent", "fs", "model", "serial"}

	return &diskCollector{
		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyGaugeMetric(prefix, "size", labelNames...).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "free", labelNames...).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "mounted", labelNames...).
				WithConverter(convert.MetricFromBool).
				Build(),
		},
		entries: metrics.NewPropertyGaugeMetric(prefix, "slot", entryLabelNames...).
			WithName("entry").
			WithConverter(convert.MetricConstantValue).
			Build(),
	}
}

func (c *diskCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *diskCollector) Collect(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/disk/print",
		"?disabled=false",
		"=.proplist=slot,type,fs-uuid,comment,size,free,mounted,mount-point,parent,fs,model,serial")
	if err != nil {
		return fmt.Errorf("fetch disk error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.WithLabelsFromMap(re.Map, "slot", "fs-uuid", "mount-point")
		if err := c.metrics.Collect(re.Map, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}

		lctx = ctx.WithLabelsFromMap(re.Map, "slot", "type", "fs-uuid", "comment", "parent", "fs", "model", "serial")
		if err := c.entries.Collect(re.Map, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}
