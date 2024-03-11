package collector

import (
	"strconv"
	"strings"

	"github.com/KarolBedkowski/routeros-go-client/proto"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("queue", newQueueCollector)
}

type queueCollector struct {
	monitorProps     []string
	monitorPropslist string

	simpleQueueProps     []string
	simpleQueuePropslist string

	descriptions map[string]*prometheus.Desc
}

func newQueueCollector() routerOSCollector {
	c := &queueCollector{
		descriptions: make(map[string]*prometheus.Desc),
	}

	c.monitorProps = []string{"queued-packets", "queued-bytes"}
	c.monitorPropslist = strings.Join(c.simpleQueueProps, ",")

	labelNames := []string{"name", "address"}
	for _, p := range c.monitorProps {
		c.descriptions[p] = descriptionForPropertyName("queue", p, labelNames)
	}

	c.simpleQueueProps = []string{
		"name", "queue", "comment",
		"disabled",
		"bytes", "packets", "queued-bytes", "queued-packets",
	}
	c.simpleQueuePropslist = strings.Join(c.simpleQueueProps, ",")

	labelNames = []string{"name", "address", "simple_queue_name", "queue", "comment"}
	c.descriptions["disabled"] = descriptionForPropertyName("simple_queue", "disabled", labelNames)

	for _, p := range c.simpleQueueProps[4:] {
		c.descriptions["tx_"+p] = descriptionForPropertyName("simple_queue", "tx_"+p+"_total", labelNames)
		c.descriptions["rx_"+p] = descriptionForPropertyName("simple_queue", "rx_"+p+"_total", labelNames)
	}

	return c
}

func (c *queueCollector) describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.descriptions {
		ch <- d
	}

	for _, d := range c.descriptions {
		ch <- d
	}
}

func (c *queueCollector) collect(ctx *collectorContext) error {
	err := c.collectQueue(ctx)
	if err != nil {
		return err
	}

	sqStats, err := c.fetchSimpleQueue(ctx)
	if err != nil {
		return err
	}

	for _, re := range sqStats {
		c.collectForSimpleQqueue(re, ctx)
	}

	return nil
}

func (c *queueCollector) collectQueue(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/queue/monitor", "=once=", "=.proplist="+c.monitorPropslist)
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching queue statistics")

		return err
	}

	if len(reply.Re) == 0 {
		return nil
	}

	for _, p := range c.monitorProps {
		c.collectMetricForProperty(p, reply.Re[0], ctx)
	}

	return nil
}

func (c *queueCollector) collectMetricForProperty(property string, re *proto.Sentence, ctx *collectorContext) {
	desc := c.descriptions[property]
	if re.Map[property] == "" {
		return
	}

	v, err := strconv.ParseFloat(re.Map[property], 64)
	if err != nil {
		log.WithFields(log.Fields{
			"property": property,
			"device":   ctx.device.Name,
			"error":    err,
		}).Error("error parsing queue metric value")

		return
	}

	ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, ctx.device.Name, ctx.device.Address)
}

func (c *queueCollector) fetchSimpleQueue(ctx *collectorContext) ([]*proto.Sentence, error) {
	reply, err := ctx.client.Run("/queue/simple/print", "=.proplist="+c.simpleQueuePropslist)
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching simple queue metrics")

		return nil, err
	}

	return reply.Re, nil
}

func (c *queueCollector) collectForSimpleQqueue(re *proto.Sentence, ctx *collectorContext) {
	for _, p := range c.simpleQueueProps[3:] {
		desc := c.descriptions[p]
		if value := re.Map[p]; value != "" {
			var (
				v     float64
				vtype prometheus.ValueType
				err   error
			)
			vtype = prometheus.CounterValue

			switch p {
			case "disabled":
				vtype = prometheus.GaugeValue
				v = parseBool(value)
			case "bytes", "packets":
				c.collectMetricForTXRXCounters(p, re.Map["name"], re.Map["queue"], re.Map["comment"], re, ctx)
				continue
			case "queued-packets", "queued-bytes":
				c.collectMetricForTXRXCounters(p, re.Map["name"], re.Map["queue"], re.Map["comment"], re, ctx)
				continue
			default:
				v, err = strconv.ParseFloat(value, 64)
				if err != nil {
					log.WithFields(log.Fields{
						"device":    ctx.device.Name,
						"interface": re.Map["name"],
						"property":  p,
						"value":     value,
						"error":     err,
					}).Error("error parsing queue metric value")
					continue
				}
			}
			ctx.ch <- prometheus.MustNewConstMetric(desc, vtype, v, ctx.device.Name, ctx.device.Address,
				re.Map["name"], re.Map["queue"], re.Map["comment"])

		}
	}
}

func (c *queueCollector) collectMetricForTXRXCounters(property, name, queue, comment string, re *proto.Sentence, ctx *collectorContext) {
	val := re.Map[property]
	if val == "" {
		return
	}

	tx, rx, err := splitStringToFloats(re.Map[property], "/")
	if err != nil {
		log.WithFields(log.Fields{
			"device":   ctx.device.Name,
			"property": property,
			"value":    re.Map[property],
			"error":    err,
		}).Error("error parsing queue metric value")
		return
	}

	desc_tx := c.descriptions["tx_"+property]
	desc_rx := c.descriptions["rx_"+property]
	ctx.ch <- prometheus.MustNewConstMetric(desc_tx, prometheus.CounterValue, tx, ctx.device.Name, ctx.device.Address, name, queue, comment)
	ctx.ch <- prometheus.MustNewConstMetric(desc_rx, prometheus.CounterValue, rx, ctx.device.Name, ctx.device.Address, name, queue, comment)
}
