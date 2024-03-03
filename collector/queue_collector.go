package collector

import (
	"strconv"
	"strings"
	"math"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"gopkg.in/routeros.v2/proto"
)

type queueCollector struct {
	monitorProps        []string
	monitorDescriptions map[string]*prometheus.Desc

	simpleQueueProps []string
	simpleQueueDescriptions map[string]*prometheus.Desc
}

func newQueueCollector() routerOSCollector {
	c := &queueCollector{}
	c.init()
	return c
}

func (c *queueCollector) init() {
	c.monitorProps = []string{"queued-packets", "queued-bytes"}
	labelNames := []string{"name", "address"}
	c.monitorDescriptions = make(map[string]*prometheus.Desc)
	for _, p := range c.monitorProps {
		c.monitorDescriptions[p] = descriptionForPropertyName("queue", p, labelNames)
	}

	c.simpleQueueProps = []string{"name", "queue", "comment", "disabled", "bytes", "packets", "queued-bytes", "queued-packets"}
	labelNames = []string{"name", "address", "simple_queue_name", "queue", "comment"}
	c.simpleQueueDescriptions = make(map[string]*prometheus.Desc)
	c.simpleQueueDescriptions["disabled"] = descriptionForPropertyName("simple_queue", "disabled", labelNames)
	for _, p := range c.simpleQueueProps[4:] {
		c.simpleQueueDescriptions["tx_"+p] = descriptionForPropertyName("simple_queue", "tx_"+p, labelNames)
		c.simpleQueueDescriptions["rx_"+p] = descriptionForPropertyName("simple_queue", "rx_"+p, labelNames)
	}
}

func (c *queueCollector) describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.monitorDescriptions {
		ch <- d
	}

	for _, d := range c.simpleQueueDescriptions {
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
	reply, err := ctx.client.Run("/queue/monitor", "=once=", "=.proplist="+strings.Join(c.monitorProps, ","))
	if err != nil {
		log.WithFields(log.Fields{
			"device":    ctx.device.Name,
			"error":     err,
		}).Error("error fetching queue statistics")
		return err
	}

	for _, p := range c.monitorProps {
		c.collectMetricForProperty(p, reply.Re[0], ctx)
	}

	return nil
}

func (c *queueCollector) collectMetricForProperty(property string, re *proto.Sentence, ctx *collectorContext) {
	desc := c.monitorDescriptions[property]
	if re.Map[property] == "" {
		return
	}
	v, err := strconv.ParseFloat(re.Map[property], 64)
	if err != nil {
		log.WithFields(log.Fields{
			"property":  property,
			"device":    ctx.device.Name,
			"error":     err,
		}).Error("error parsing queue metric value")
		return
	}

	ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, ctx.device.Name, ctx.device.Address)
}

func (c *queueCollector) fetchSimpleQueue(ctx *collectorContext) ([]*proto.Sentence, error) {
	reply, err := ctx.client.Run("/queue/simple/print", "=.proplist="+strings.Join(c.simpleQueueProps, ","))
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
		desc := c.simpleQueueDescriptions[p]
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
				if value == "true" {
					v = 1
				} else {
					v = 0
				}
			case "bytes":
				c.collectMetricForTXRXCounters(p, re.Map["name"], re.Map["queue"], re.Map["comment"], re, ctx)
				continue
			case "packets":
				c.collectMetricForTXRXCounters(p, re.Map["name"], re.Map["queue"], re.Map["comment"], re, ctx)
				continue
			case "queued-packets":
				c.collectMetricForTXRXCounters("queued-packets", re.Map["name"], re.Map["queue"], re.Map["comment"], re, ctx)
				continue
			case "queued-bytes":
				c.collectMetricForTXRXCounters("queued-bytes", re.Map["name"], re.Map["queue"], re.Map["comment"], re, ctx)
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
	tx, rx, err := splitToFloats(re.Map[property])
	if err != nil {
		log.WithFields(log.Fields{
			"device":   ctx.device.Name,
			"property": property,
			"value":    re.Map[property],
			"error":    err,
		}).Error("error parsing queue metric value")
		return
	}
	desc_tx := c.simpleQueueDescriptions["tx_"+property]
	desc_rx := c.simpleQueueDescriptions["rx_"+property]
	ctx.ch <- prometheus.MustNewConstMetric(desc_tx, prometheus.CounterValue, tx, ctx.device.Name, ctx.device.Address, name, queue, comment)
	ctx.ch <- prometheus.MustNewConstMetric(desc_rx, prometheus.CounterValue, rx, ctx.device.Name, ctx.device.Address, name, queue, comment)
}

func splitToFloats(metric string) (float64, float64, error) {
	if metric  == "" {
		return 0, 0, nil
	}
	strs := strings.Split(metric, "/")
	if len(strs) != 2 {
		return 0, 0, nil
	}
	var m1, m2 float64
	var err error

	if strs[0]!= "" {
		m1, err = strconv.ParseFloat(strs[0], 64)
		if err != nil {
			return math.NaN(), math.NaN(), err
		}
	}

	if strs[1] != "" {
		m2, err = strconv.ParseFloat(strs[1], 64)
		if err != nil {
			return math.NaN(), math.NaN(), err
		}
	}

	return m1, m2, nil
}
