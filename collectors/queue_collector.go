package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("queue", newQueueCollector,
		"retrieves Simple Queue and queue monitor metrics")
}

type queueCollector struct {
	metrics PropertyMetricList

	monitorQueuedBytes   PropertyMetric
	monitorQueuedPackets PropertyMetric
}

func newQueueCollector() RouterOSCollector {
	monitorLabelNames := []string{"name", "address"}
	labelNames := []string{"name", "address", "simple_queue_name", "queue", "comment"}

	const sqPrefix = "simple_queue"

	return &queueCollector{
		monitorQueuedBytes:   NewPropertyGaugeMetric("queue", "queued-bytes", monitorLabelNames).Build(),
		monitorQueuedPackets: NewPropertyGaugeMetric("queue", "queued-packets", monitorLabelNames).Build(),

		metrics: PropertyMetricList{
			NewPropertyGaugeMetric(sqPrefix, "disabled", labelNames).WithConverter(metricFromBool).Build(),
			NewPropertyRxTxMetric(sqPrefix, "packets", labelNames).WithRxTxConverter(metricFromQueueTxRx).Build(),
			NewPropertyRxTxMetric(sqPrefix, "bytes", labelNames).WithRxTxConverter(metricFromQueueTxRx).Build(),
			NewPropertyRxTxMetric(sqPrefix, "queued-packets", labelNames).WithRxTxConverter(metricFromQueueTxRx).Build(),
			NewPropertyRxTxMetric(sqPrefix, "queued-bytes", labelNames).WithRxTxConverter(metricFromQueueTxRx).Build(),
		},
	}
}

func (c *queueCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
	c.monitorQueuedBytes.Describe(ch)
	c.monitorQueuedPackets.Describe(ch)
}

func (c *queueCollector) Collect(ctx *CollectorContext) error {
	return multierror.Append(nil,
		c.collectQueue(ctx),
		c.collectSimpleQueue(ctx),
	).ErrorOrNil()
}

func metricFromQueueTxRx(value string) (float64, float64, error) {
	return splitStringToFloats(value, "/")
}

func (c *queueCollector) collectQueue(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/queue/monitor", "=once=", "=.proplist=queued-packets,queued-bytes")
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

func (c *queueCollector) collectSimpleQueue(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/queue/simple/print",
		"=.proplist=name,queue,comment,disabled,bytes,packets,queued-bytes,queued-packets")
	if err != nil {
		return fmt.Errorf("fetch simple queue error: %w", err)
	}

	var errs *multierror.Error

	for _, reply := range reply.Re {
		name := reply.Map["name"]
		queue := reply.Map["queue"]
		ctx = ctx.withLabels(name, queue, reply.Map["comment"])

		if err := c.metrics.Collect(reply, ctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect %v/%v error: %w", name, queue, err))
		}
	}

	return errs.ErrorOrNil()
}
