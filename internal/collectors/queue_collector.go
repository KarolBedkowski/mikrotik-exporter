package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("queue", newQueueCollector,
		"retrieves Simple Queue and queue monitor metrics")
}

type queueCollector struct {
	monitorQueuedBytes   metrics.PropertyMetric
	monitorQueuedPackets metrics.PropertyMetric
	metrics              metrics.PropertyMetricList
}

func newQueueCollector() RouterOSCollector {
	labelNames := []string{"simple_queue_name", "queue", metrics.LabelComment}

	const sqPrefix = "simple_queue"

	return &queueCollector{
		monitorQueuedBytes:   metrics.NewPropertyGaugeMetric("queue", "queued-bytes").Build(),
		monitorQueuedPackets: metrics.NewPropertyGaugeMetric("queue", "queued-packets").Build(),

		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyRxTxMetric(sqPrefix, "packets", labelNames...).WithRxTxConverter(metricFromQueueTxRx).Build(),
			metrics.NewPropertyRxTxMetric(sqPrefix, "bytes", labelNames...).WithRxTxConverter(metricFromQueueTxRx).Build(),
			metrics.NewPropertyRxTxMetric(sqPrefix, "queued-packets", labelNames...).WithRxTxConverter(metricFromQueueTxRx).Build(),
			metrics.NewPropertyRxTxMetric(sqPrefix, "queued-bytes", labelNames...).WithRxTxConverter(metricFromQueueTxRx).Build(),
		},
	}
}

func (c *queueCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
	c.monitorQueuedBytes.Describe(ch)
	c.monitorQueuedPackets.Describe(ch)
}

func (c *queueCollector) Collect(ctx *metrics.CollectorContext) error {
	return multierror.Append(nil,
		c.collectQueue(ctx),
		c.collectSimpleQueue(ctx),
	).ErrorOrNil()
}

func metricFromQueueTxRx(value string) (float64, float64, error) {
	return metrics.SplitStringToFloats(value, "/")
}

func (c *queueCollector) collectQueue(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/queue/monitor", "=once=", "=.proplist=queued-packets,queued-bytes")
	if err != nil {
		return fmt.Errorf("fetch queue monitor error: %w", err)
	}

	if len(reply.Re) == 0 {
		return nil
	}

	re := reply.Re[0]

	if err := c.monitorQueuedBytes.Collect(re, ctx); err != nil {
		return fmt.Errorf("collect queue monitor error: %w", err)
	}

	if err := c.monitorQueuedPackets.Collect(re, ctx); err != nil {
		return fmt.Errorf("collect queue monitor error: %w", err)
	}

	return nil
}

func (c *queueCollector) collectSimpleQueue(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/queue/simple/print",
		"?disabled=false",
		"=.proplist=name,queue,comment,bytes,packets,queued-bytes,queued-packets")
	if err != nil {
		return fmt.Errorf("fetch simple queue error: %w", err)
	}

	var errs *multierror.Error

	for _, reply := range reply.Re {
		lctx := ctx.WithLabelsFromMap(reply.Map, "name", "queue", "comment")

		if err := c.metrics.Collect(reply, &lctx); err != nil {
			name := reply.Map["name"]
			queue := reply.Map["queue"]
			errs = multierror.Append(errs, fmt.Errorf("collect %v/%v error: %w", name, queue, err))
		}
	}

	return errs.ErrorOrNil()
}
