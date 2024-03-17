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
	simpleQueueProps     []string
	simpleQueuePropslist string

	monitorQueuedBytesDesc   *prometheus.Desc
	monitorQueuedPacketsDesc *prometheus.Desc
	packetsDesc              *TXRXDecription
	bytesDesc                *TXRXDecription
	queuedBytesDesc          *TXRXDecription
	queuedPacketsDesc        *TXRXDecription
	disabledDesc             *prometheus.Desc
}

func newQueueCollector() routerOSCollector {
	monitorLabelNames := []string{"name", "address"}
	labelNames := []string{"name", "address", "simple_queue_name", "queue", "comment"}

	collector := &queueCollector{
		monitorQueuedBytesDesc:   descriptionForPropertyName("queue", "queue-bytes", monitorLabelNames),
		monitorQueuedPacketsDesc: descriptionForPropertyName("queue", "queue-packets", monitorLabelNames),
		disabledDesc:             descriptionForPropertyName("simple_queue", "disabled", labelNames),
		packetsDesc:              NewTXRXDescription("simple_queue", "packets_total", labelNames),
		bytesDesc:                NewTXRXDescription("simple_queue", "bytes_total", labelNames),
		queuedPacketsDesc:        NewTXRXDescription("simple_queue", "queued_packets_total", labelNames),
		queuedBytesDesc:          NewTXRXDescription("simple_queue", "queued_bytes_total", labelNames),
	}

	collector.simpleQueueProps = []string{
		"name", "queue", "comment",
		"disabled",
		"bytes", "packets", "queued-bytes", "queued-packets",
	}
	collector.simpleQueuePropslist = strings.Join(collector.simpleQueueProps, ",")

	return collector
}

func (c *queueCollector) describe(ch chan<- *prometheus.Desc) {
	c.packetsDesc.describe(ch)
	c.bytesDesc.describe(ch)
	c.queuedBytesDesc.describe(ch)
	c.queuedPacketsDesc.describe(ch)
	ch <- c.disabledDesc
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
		pcl := newPropertyCollector(reply, ctx, reply.Map["name"], reply.Map["queue"], reply.Map["comment"])

		_ = pcl.collectGaugeValue(c.disabledDesc, "disabled", convertFromBool)
		_ = pcl.collectRXTXCounterValue(c.bytesDesc, "bytes", queueTxRxConverter)
		_ = pcl.collectRXTXCounterValue(c.packetsDesc, "packets", queueTxRxConverter)
		_ = pcl.collectRXTXCounterValue(c.queuedBytesDesc, "queued-bytes", queueTxRxConverter)
		_ = pcl.collectRXTXCounterValue(c.queuedPacketsDesc, "queued-packets", queueTxRxConverter)
	}

	return nil
}
