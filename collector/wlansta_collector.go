package collector

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("wlansta", newWlanSTACollector)
}

type wlanSTACollector struct {
	propslist    string
	descriptions map[string]*prometheus.Desc
}

func newWlanSTACollector() routerOSCollector {
	collector := &wlanSTACollector{
		descriptions: make(map[string]*prometheus.Desc),
	}

	props := []string{"interface", "mac-address", "signal-to-noise", "signal-strength", "packets", "bytes", "frames"}
	collector.propslist = strings.Join(props, ",")

	labelNames := []string{"name", "address", "interface", "mac_address"}

	collector.descriptions["signal-to-noise"] = descriptionForPropertyName("wlan_station", "signal-to-noise", labelNames)
	collector.descriptions["signal-strength"] = descriptionForPropertyName("wlan_station", "signal-strength", labelNames)

	for _, p := range []string{"packets", "bytes", "frames"} {
		collector.descriptions["tx_"+p] = descriptionForPropertyName("wlan_station", "tx_"+p, labelNames)
		collector.descriptions["rx_"+p] = descriptionForPropertyName("wlan_station", "rx_"+p, labelNames)
	}

	return collector
}

func (c *wlanSTACollector) describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.descriptions {
		ch <- d
	}
}

func (c *wlanSTACollector) collect(ctx *collectorContext) error {
	stats, err := c.fetch(ctx)
	if err != nil {
		return err
	}

	for _, re := range stats {
		c.collectForStat(re, ctx)
	}

	return nil
}

func (c *wlanSTACollector) fetch(ctx *collectorContext) ([]*proto.Sentence, error) {
	reply, err := ctx.client.Run("/interface/wireless/registration-table/print", "=.proplist="+c.propslist)
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching wlan station metrics")

		return nil, fmt.Errorf("read wireless reg error: %w", err)
	}

	return reply.Re, nil
}

func (c *wlanSTACollector) collectForStat(re *proto.Sentence, ctx *collectorContext) {
	iface := re.Map["interface"]
	mac := re.Map["mac-address"]

	c.collectMetricForProperty("signal-to-noise", iface, mac, re, ctx)
	c.collectMetricForProperty("signal-strength", iface, mac, re, ctx)

	c.collectMetricForTXRXCounters("packets", iface, mac, re, ctx)
	c.collectMetricForTXRXCounters("bytes", iface, mac, re, ctx)
	c.collectMetricForTXRXCounters("frames", iface, mac, re, ctx)
}

func (c *wlanSTACollector) collectMetricForProperty(
	property, iface, mac string, reply *proto.Sentence, ctx *collectorContext,
) {
	if reply.Map[property] == "" {
		return
	}

	p := reply.Map[property]
	if i := strings.Index(p, "@"); i > -1 {
		p = p[:i]
	}

	value, err := strconv.ParseFloat(p, 64)
	if err != nil {
		log.WithFields(log.Fields{
			"device":   ctx.device.Name,
			"property": property,
			"value":    reply.Map[property],
			"error":    err,
		}).Error("error parsing wlan station metric value")

		return
	}

	desc := c.descriptions[property]
	ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
		value, ctx.device.Name, ctx.device.Address, iface, mac)
}

func (c *wlanSTACollector) collectMetricForTXRXCounters(
	property, iface, mac string, re *proto.Sentence, ctx *collectorContext,
) {
	tx, rx, err := splitStringToFloats(re.Map[property])
	if err != nil {
		log.WithFields(log.Fields{
			"device":   ctx.device.Name,
			"property": property,
			"value":    re.Map[property],
			"error":    err,
		}).Error("error parsing wlan station metric value")

		return
	}

	descTX := c.descriptions["tx_"+property]
	ctx.ch <- prometheus.MustNewConstMetric(
		descTX, prometheus.CounterValue, tx, ctx.device.Name, ctx.device.Address, iface, mac)

	descRX := c.descriptions["rx_"+property]
	ctx.ch <- prometheus.MustNewConstMetric(
		descRX, prometheus.CounterValue, rx, ctx.device.Name, ctx.device.Address, iface, mac)
}
