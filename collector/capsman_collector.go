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
	registerCollector("capsman", newCapsmanCollector)
}

type capsmanCollector struct {
	proplist     string
	descriptions map[string]*prometheus.Desc

	radioProplist         string
	radiosProvisionedDesc *prometheus.Desc
}

func newCapsmanCollector() routerOSCollector {
	collector := &capsmanCollector{
		descriptions: make(map[string]*prometheus.Desc, 7),
	}

	props := []string{"interface", "mac-address", "ssid", "uptime", "tx-signal", "rx-signal", "packets", "bytes"}
	collector.proplist = strings.Join(props, ",")

	labelNames := []string{"name", "address", "interface", "mac_address", "ssid"}

	collector.descriptions["uptime"] = descriptionForPropertyName("capsman_station", "uptime", labelNames)
	collector.descriptions["tx-signal"] = descriptionForPropertyName("capsman_station", "tx-signal", labelNames)
	collector.descriptions["rx-signal"] = descriptionForPropertyName("capsman_station", "rx-signal", labelNames)
	collector.descriptions["tx_packets"] = descriptionForPropertyName("capsman_station", "tx_packets_total", labelNames)
	collector.descriptions["rx_packets"] = descriptionForPropertyName("capsman_station", "rx_packets_total", labelNames)
	collector.descriptions["tx_bytes"] = descriptionForPropertyName("capsman_station", "tx_bytes_total", labelNames)
	collector.descriptions["rx_bytes"] = descriptionForPropertyName("capsman_station", "rx_bytes_total", labelNames)

	radioProps := []string{"interface", "radio-mac", "remote-cap-identity", "remote-cap-name", "provisioned"}
	collector.radioProplist = strings.Join(radioProps, ",")
	labelNames = []string{"name", "address", "interface", "radio_mac", "remote_cap_identity", "remote_cap_name"}
	collector.radiosProvisionedDesc = description("capsman", "radio_provisioned",
		"Status of provision remote radios", labelNames)

	return collector
}

func (c *capsmanCollector) describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.descriptions {
		ch <- d
	}

	ch <- c.radiosProvisionedDesc
}

func (c *capsmanCollector) collect(ctx *collectorContext) error {
	stats, err := c.fetch(ctx)
	if err != nil {
		return err
	}

	for _, re := range stats {
		c.collectForStat(re, ctx)
	}

	err = c.collectRadiosProvisioned(ctx)

	return err
}

func (c *capsmanCollector) fetch(ctx *collectorContext) ([]*proto.Sentence, error) {
	reply, err := ctx.client.Run("/caps-man/registration-table/print", "=.proplist="+c.proplist)
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching wlan station metrics")

		return nil, fmt.Errorf("get capsman reg error: %w", err)
	}

	return reply.Re, nil
}

func (c *capsmanCollector) collectForStat(re *proto.Sentence, ctx *collectorContext) {
	iface := re.Map["interface"]
	mac := re.Map["mac-address"]
	ssid := re.Map["ssid"]

	c.collectMetricForProperty("uptime", iface, mac, ssid, re, ctx)
	c.collectMetricForProperty("tx-signal", iface, mac, ssid, re, ctx)
	c.collectMetricForProperty("rx-signal", iface, mac, ssid, re, ctx)

	c.collectMetricForTXRXCounters("packets", iface, mac, ssid, re, ctx)
	c.collectMetricForTXRXCounters("bytes", iface, mac, ssid, re, ctx)
}

func (c *capsmanCollector) collectMetricForProperty(
	property, iface, mac, ssid string, reply *proto.Sentence, ctx *collectorContext,
) {
	if reply.Map[property] == "" {
		return
	}

	propertyVal := reply.Map[property]
	if i := strings.Index(propertyVal, "@"); i > -1 {
		propertyVal = propertyVal[:i]
	}

	var (
		value float64
		err   error
	)

	if property == "uptime" {
		value, err = parseDuration(propertyVal)
	} else {
		value, err = strconv.ParseFloat(propertyVal, 64)
	}

	if err != nil {
		log.WithFields(log.Fields{
			"device":   ctx.device.Name,
			"property": property,
			"value":    reply.Map[property],
			"error":    err,
		}).Error("error parsing capsman station metric value")

		return
	}

	desc := c.descriptions[property]
	ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
		value, ctx.device.Name, ctx.device.Address, iface, mac, ssid)
}

func (c *capsmanCollector) collectMetricForTXRXCounters(
	property, iface, mac, ssid string, re *proto.Sentence, ctx *collectorContext,
) {
	tx, rx, err := splitStringToFloats(re.Map[property])
	if err != nil {
		log.WithFields(log.Fields{
			"device":   ctx.device.Name,
			"property": property,
			"value":    re.Map[property],
			"error":    err,
		}).Error("error parsing capsman station metric value")

		return
	}

	descTX := c.descriptions["tx_"+property]
	ctx.ch <- prometheus.MustNewConstMetric(
		descTX, prometheus.CounterValue, tx, ctx.device.Name, ctx.device.Address, iface, mac, ssid)
	descRX := c.descriptions["rx_"+property]
	ctx.ch <- prometheus.MustNewConstMetric(
		descRX, prometheus.CounterValue, rx, ctx.device.Name, ctx.device.Address, iface, mac, ssid)
}

func (c *capsmanCollector) collectRadiosProvisioned(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/caps-man/radio/print", "=.proplist="+c.radioProplist)
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching capsman radios metrics")

		return fmt.Errorf("get capsman radio error: %w", err)
	}

	for _, re := range reply.Re {
		v := parseBool(re.Map["provisioned"])
		ctx.ch <- prometheus.MustNewConstMetric(c.radiosProvisionedDesc,
			prometheus.GaugeValue, v, ctx.device.Name, ctx.device.Address,
			re.Map["interface"], re.Map["radio-mac"], re.Map["remote-cap-identity"], re.Map["remote-cap-name"])
	}

	return nil
}
