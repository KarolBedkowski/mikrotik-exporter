package collector

import (
	"strconv"
	"strings"

	"github.com/go-routeros/routeros/proto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type capsmanCollector struct {
	props        []string
	descriptions map[string]*prometheus.Desc

	radioProps            []string
	radiosProvisionedDesc *prometheus.Desc
}

func newCapsmanCollector() routerOSCollector {
	c := &capsmanCollector{}
	c.init()
	return c
}

func (c *capsmanCollector) init() {
	//"rx-signal", "tx-signal",
	c.props = []string{"interface", "mac-address", "ssid", "uptime", "tx-signal", "rx-signal", "packets", "bytes"}
	labelNames := []string{"name", "address", "interface", "mac_address", "ssid"}
	c.descriptions = make(map[string]*prometheus.Desc)
	for _, p := range c.props[3 : len(c.props)-2] {
		c.descriptions[p] = descriptionForPropertyName("capsman_station", p, labelNames)
	}
	for _, p := range c.props[len(c.props)-2:] {
		c.descriptions["tx_"+p] = descriptionForPropertyName("capsman_station", "tx_"+p, labelNames)
		c.descriptions["rx_"+p] = descriptionForPropertyName("capsman_station", "rx_"+p, labelNames)
	}

	c.radioProps = []string{"interface", "radio-mac", "remote-cap-identity", "remote-cap-name", "provisioned"}
	labelNames = []string{"name", "address", "interface", "radio_mac", "remote_cap_identity", "remote_cap_name"}
	c.radiosProvisionedDesc = description("capsman", "radio_provisioned", "Status of provision remote radios", labelNames)
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
	if err != nil {
		return err
	}

	return nil
}

func (c *capsmanCollector) fetch(ctx *collectorContext) ([]*proto.Sentence, error) {
	reply, err := ctx.client.Run("/caps-man/registration-table/print", "=.proplist="+strings.Join(c.props, ","))
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching wlan station metrics")
		return nil, err
	}

	return reply.Re, nil
}

func (c *capsmanCollector) collectForStat(re *proto.Sentence, ctx *collectorContext) {
	iface := re.Map["interface"]
	mac := re.Map["mac-address"]
	ssid := re.Map["ssid"]

	for _, p := range c.props[3 : len(c.props)-2] {
		c.collectMetricForProperty(p, iface, mac, ssid, re, ctx)
	}
	for _, p := range c.props[len(c.props)-2:] {
		c.collectMetricForTXRXCounters(p, iface, mac, ssid, re, ctx)
	}
}

func (c *capsmanCollector) collectMetricForProperty(property, iface, mac, ssid string, re *proto.Sentence, ctx *collectorContext) {
	if re.Map[property] == "" {
		return
	}
	p := re.Map[property]
	i := strings.Index(p, "@")
	if i > -1 {
		p = p[:i]
	}
	var v float64
	var err error
	if property != "uptime" {
		v, err = strconv.ParseFloat(p, 64)
	} else {
		v, err = parseDuration(p)
	}
	if err != nil {
		log.WithFields(log.Fields{
			"device":   ctx.device.Name,
			"property": property,
			"value":    re.Map[property],
			"error":    err,
		}).Error("error parsing capsman station metric value")
		return
	}

	desc := c.descriptions[property]
	ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, ctx.device.Name, ctx.device.Address, iface, mac, ssid)
}

func (c *capsmanCollector) collectMetricForTXRXCounters(property, iface, mac, ssid string, re *proto.Sentence, ctx *collectorContext) {
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
	desc_tx := c.descriptions["tx_"+property]
	desc_rx := c.descriptions["rx_"+property]
	ctx.ch <- prometheus.MustNewConstMetric(desc_tx, prometheus.CounterValue, tx, ctx.device.Name, ctx.device.Address, iface, mac, ssid)
	ctx.ch <- prometheus.MustNewConstMetric(desc_rx, prometheus.CounterValue, rx, ctx.device.Name, ctx.device.Address, iface, mac, ssid)
}

func (c *capsmanCollector) collectRadiosProvisioned(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/caps-man/radio/print", "=.proplist="+strings.Join(c.radioProps, ","))
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching capsman radios metrics")
		return err
	}

	for _, re := range reply.Re {
		iface := re.Map["interface"]
		rmac := re.Map["radio-mac"]
		rcIdent := re.Map["remote-cap-identity"]
		rcName := re.Map["remote-cap-name"]
		v := 0.0
		if re.Map["provisioned"] == "true" {
			v = 1.0
		}

		ctx.ch <- prometheus.MustNewConstMetric(c.radiosProvisionedDesc,
			prometheus.GaugeValue, v, ctx.device.Name, ctx.device.Address,
			iface, rmac, rcIdent, rcName)
	}

	return nil
}
