package collector

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("queue", newQueueCollector)
}

type queueCollector struct {
	metrics propertyMetricList

	monitorQueuedBytes   propertyMetricCollector
	monitorQueuedPackets propertyMetricCollector
}

func newQueueCollector() routerOSCollector {
	monitorLabelNames := []string{"name", "address"}
	labelNames := []string{"name", "address", "simple_queue_name", "queue", "comment"}

	const sqPrefix = "simple_queue"

	collector := &queueCollector{
		monitorQueuedBytes:   newPropertyGaugeMetric("queue", "queued-bytes", monitorLabelNames).build(),
		monitorQueuedPackets: newPropertyGaugeMetric("queue", "queued-packets", monitorLabelNames).build(),

		metrics: propertyMetricList{
			newPropertyGaugeMetric(sqPrefix, "disabled", labelNames).withConverter(convertFromBool).build(),
			newPropertyRxTxMetric(sqPrefix, "packets", labelNames).withRxTxConverter(queueTxRxConverter).build(),
			newPropertyRxTxMetric(sqPrefix, "bytes", labelNames).withRxTxConverter(queueTxRxConverter).build(),
			newPropertyRxTxMetric(sqPrefix, "queued-packets", labelNames).withRxTxConverter(queueTxRxConverter).build(),
			newPropertyRxTxMetric(sqPrefix, "queued-bytes", labelNames).withRxTxConverter(queueTxRxConverter).build(),
		},
	}

	return collector
}

func (c *queueCollector) describe(ch chan<- *prometheus.Desc) {
	c.metrics.describe(ch)
	c.monitorQueuedBytes.describe(ch)
	c.monitorQueuedPackets.describe(ch)
}

func (c *queueCollector) collect(ctx *collectorContext) error {
	if err := c.collectQueue(ctx); err != nil {
		return err
	}

	return c.collectSimpleQueue(ctx)
}

func queueTxRxConverter(value string) (float64, float64, error) {
	return splitStringToFloats(value, "/")
}

func (c *queueCollector) collectQueue(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/queue/monitor", "=once=", "=.proplist=queued-packets,queued-bytes")
	if err != nil {
		return fmt.Errorf("fetch queue monitor error: %w", err)
	}

	if len(reply.Re) == 0 {
		return nil
	}

	re := reply.Re[0]

	if err := c.monitorQueuedBytes.collect(re, ctx); err != nil {
		return fmt.Errorf("collect queue monitor error: %w", err)
	}

	if err := c.monitorQueuedPackets.collect(re, ctx); err != nil {
		return fmt.Errorf("collect queue monitor error: %w", err)
	}

	return nil
}

func (c *queueCollector) collectSimpleQueue(ctx *collectorContext) error {
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

		if err := c.metrics.collect(reply, ctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect %v/%verror %w", name, queue, err))
		}
	}

	return errs.ErrorOrNil()
}
