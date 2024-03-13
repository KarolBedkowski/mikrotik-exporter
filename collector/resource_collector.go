package collector

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var (
	uptimeRegex *regexp.Regexp
	uptimeParts [5]time.Duration
)

func init() {
	registerCollector("resource", newResourceCollector)

	uptimeRegex = regexp.MustCompile(`(?:(\d*)w)?(?:(\d*)d)?(?:(\d*)h)?(?:(\d*)m)?(?:(\d*)s)?`)
	uptimeParts = [5]time.Duration{time.Hour * 168, time.Hour * 24, time.Hour, time.Minute, time.Second}
}

type resourceCollector struct {
	props        []string
	descriptions map[string]*prometheus.Desc
	versions     *prometheus.Desc
}

func newResourceCollector() routerOSCollector {
	collector := &resourceCollector{
		descriptions: make(map[string]*prometheus.Desc),
	}

	collector.props = []string{
		"free-memory", "total-memory", "cpu-load", "free-hdd-space", "total-hdd-space", "cpu-frequency", "bad-blocks",
		"uptime", "cpu-count",
		"board-name", "version",
	}

	labelNames := []string{"name", "address"}

	for _, p := range collector.props[:len(collector.props)-4] {
		collector.descriptions[p] = descriptionForPropertyName("system", p, labelNames)
	}

	collector.descriptions["cpu-count"] = descriptionForPropertyName("system", "cpu", labelNames)
	collector.descriptions["uptime"] = descriptionForPropertyName("system", "uptime_total", labelNames)
	collector.versions = description("system", "routeros", "Board and system version",
		[]string{"name", "address", "board_name", "version"})

	return collector
}

func (c *resourceCollector) describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.descriptions {
		ch <- d
	}

	ch <- c.versions
}

func (c *resourceCollector) collect(ctx *collectorContext) error {
	stats, err := c.fetch(ctx)
	if err != nil {
		return err
	}

	for _, re := range stats {
		c.collectForStat(re, ctx)
	}

	return nil
}

func (c *resourceCollector) fetch(ctx *collectorContext) ([]*proto.Sentence, error) {
	reply, err := ctx.client.Run("/system/resource/print", "=.proplist="+strings.Join(c.props, ","))
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching system resource metrics")

		return nil, fmt.Errorf("get resource error: %w", err)
	}

	return reply.Re, nil
}

func (c *resourceCollector) collectForStat(re *proto.Sentence, ctx *collectorContext) {
	boardname := re.Map["board-name"]
	version := re.Map["version"]

	ctx.ch <- prometheus.MustNewConstMetric(c.versions, prometheus.GaugeValue, 1,
		ctx.device.Name, ctx.device.Address, boardname, version)

	for _, p := range c.props[:9] {
		c.collectMetricForProperty(p, re, ctx)
	}
}

func (c *resourceCollector) collectMetricForProperty(property string, reply *proto.Sentence, ctx *collectorContext) {
	var (
		metricValue float64
		vtype       = prometheus.GaugeValue
		err         error
	)

	if property == "uptime" {
		metricValue, err = parseUptime(reply.Map[property])
		vtype = prometheus.CounterValue
	} else {
		if reply.Map[property] == "" {
			return
		}

		metricValue, err = strconv.ParseFloat(reply.Map[property], 64)
	}

	if err != nil {
		log.WithFields(log.Fields{
			"device":   ctx.device.Name,
			"property": property,
			"value":    reply.Map[property],
			"error":    err,
		}).Error("error parsing system resource metric value")

		return
	}

	desc := c.descriptions[property]
	ctx.ch <- prometheus.MustNewConstMetric(desc, vtype, metricValue, ctx.device.Name, ctx.device.Address)
}

var ErrInvalidUptime = errors.New("invalid uptime value sent to regex")

// TODO:  duplicated?
func parseUptime(uptime string) (float64, error) {
	var totalUptime time.Duration

	reMatch := uptimeRegex.FindAllStringSubmatch(uptime, -1)

	// should get one and only one match back on the regex
	if len(reMatch) != 1 {
		return 0, ErrInvalidUptime
	}

	for idx, match := range reMatch[0][1:] {
		if match != "" {
			v, err := strconv.Atoi(match)
			if err != nil {
				log.WithFields(log.Fields{
					"uptime": uptime,
					"value":  match,
					"error":  err,
				}).Error("error parsing uptime field value")

				return float64(0), err
			}

			totalUptime += time.Duration(v) * uptimeParts[idx]
		}
	}

	return totalUptime.Seconds(), nil
}
