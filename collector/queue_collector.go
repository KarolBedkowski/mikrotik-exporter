package collector

import (
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("queue", newQueueCollector)
}

type queueCollector struct {
	simpleQueuePropslist string

	metrics []propertyMetricCollector

	monitorQueuedBytesDesc   *prometheus.Desc
	monitorQueuedPacketsDesc *prometheus.Desc
}

func newQueueCollector() routerOSCollector {
	monitorLabelNames := []string{"name", "address"}
	labelNames := []string{"name", "address", "simple_queue_name", "queue", "comment"}

	const sqPrefix = "simple_queue"

	collector := &queueCollector{
		monitorQueuedBytesDesc:   descriptionForPropertyName("queue", "queue-bytes", monitorLabelNames),
		monitorQueuedPacketsDesc: descriptionForPropertyName("queue", "queue-packets", monitorLabelNames),

		metrics: []propertyMetricCollector{
			newPropertyGaugeMetric(sqPrefix, "disabled", labelNames).withConverter(convertFromBool).build(),
			newPropertyRxTxMetric(sqPrefix, "packets", labelNames).withRxTxConverter(queueTxRxConverter).build(),
			newPropertyRxTxMetric(sqPrefix, "bytes", labelNames).withRxTxConverter(queueTxRxConverter).build(),
			newPropertyRxTxMetric(sqPrefix, "queued_packets", labelNames).withRxTxConverter(queueTxRxConverter).build(),
			newPropertyRxTxMetric(sqPrefix, "queued_bytes", labelNames).withRxTxConverter(queueTxRxConverter).build(),
		},
	}

	simpleQueueProps := []string{
		"name", "queue", "comment",
		"disabled",
		"bytes", "packets", "queued-bytes", "queued-packets",
	}
	collector.simpleQueuePropslist = strings.Join(simpleQueueProps, ",")

	return collector
}

func (c *queueCollector) describe(ch chan<- *prometheus.Desc) {
	for _, c := range c.metrics {
		c.describe(ch)
	}

	ch <- c.monitorQueuedBytesDesc
	ch <- c.monitorQueuedPacketsDesc
}

func (c *queueCollector) collect(ctx *collectorContext) error {
	if err := c.collectQueue(ctx); err != nil {
		return err
	}

	return c.collectSimpleQueue(ctx)
}

func queueTxRxConverter(value string, opts ...string) (float64, float64, error) {
	return splitStringToFloats(value, "/")
}

func (c *queueCollector) collectQueue(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/queue/monitor", "=once=", "=.proplist=queued-packets,queued-bytes")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching queue statistics")

		return fmt.Errorf("get queue monitor error: %w", err)
	}

	if len(reply.Re) == 0 {
		return nil
	}

	re := reply.Re[0]
	pcl := newPropertyCollector(re, ctx)
	_ = pcl.collectGaugeValue(c.monitorQueuedBytesDesc, "queued-bytes", nil)
	_ = pcl.collectGaugeValue(c.monitorQueuedPacketsDesc, "queued-packets", nil)

	return nil
}

func (c *queueCollector) collectSimpleQueue(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/queue/simple/print", "=.proplist="+c.simpleQueuePropslist)
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching simple queue metrics")

		return fmt.Errorf("get simple queue error: %w", err)
	}

	for _, reply := range reply.Re {
		labels := []string{reply.Map["name"], reply.Map["queue"], reply.Map["comment"]}
		for _, m := range c.metrics {
			_ = m.collect(reply, ctx, labels)
		}
	}

	return nil
}
