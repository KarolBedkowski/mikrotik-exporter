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
	registerCollector("wlanif", newWlanIFCollector)
}

type wlanIFCollector struct {
	props        []string
	propslist    string
	descriptions map[string]*prometheus.Desc
	frequency    *prometheus.Desc
}

func newWlanIFCollector() routerOSCollector {
	collector := &wlanIFCollector{
		descriptions: make(map[string]*prometheus.Desc),
	}

	collector.props = []string{"registered-clients", "noise-floor", "overall-tx-ccq"}
	collector.propslist = strings.Join(append(collector.props, "channel"), ",")
	collector.frequency = description("wlan_interface", "frequency",
		"WiFi frequency", []string{"name", "address", "interface", "freqidx"})

	labelNames := []string{"name", "address", "interface", "channel"}

	for _, p := range collector.props {
		collector.descriptions[p] = descriptionForPropertyName("wlan_interface", p, labelNames)
	}

	return collector
}

func (c *wlanIFCollector) describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.descriptions {
		ch <- d
	}

	ch <- c.frequency
}

func (c *wlanIFCollector) collect(ctx *collectorContext) error {
	names, err := c.fetchInterfaceNames(ctx)
	if err != nil {
		return err
	}

	for _, n := range names {
		err := c.collectForInterface(n, ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *wlanIFCollector) fetchInterfaceNames(ctx *collectorContext) ([]string, error) {
	reply, err := ctx.client.Run("/interface/wireless/print", "=.proplist=name")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching wireless interface names")

		return nil, fmt.Errorf("read wireless error: %w", err)
	}

	names := make([]string, 0, len(reply.Re))
	for _, re := range reply.Re {
		names = append(names, re.Map["name"])
	}

	return names, nil
}

func (c *wlanIFCollector) collectForInterface(iface string, ctx *collectorContext) error {
	reply, err := ctx.client.Run("/interface/wireless/monitor", "=numbers="+iface, "=once=", "=.proplist="+c.propslist)
	if err != nil {
		log.WithFields(log.Fields{
			"interface": iface,
			"device":    ctx.device.Name,
			"error":     err,
		}).Error("error fetching interface statistics")

		return fmt.Errorf("get wireless monitor error: %w", err)
	}

	if len(reply.Re) == 0 {
		return nil
	}

	for _, p := range c.props {
		// there's always going to be only one sentence in reply, as we
		// have to explicitly specify the interface
		c.collectMetricForProperty(p, iface, reply.Re[0], ctx)
	}

	c.collectMetricForFreq(iface, reply.Re[0], ctx)

	return nil
}

func (c *wlanIFCollector) collectMetricForProperty(
	property, iface string, reply *proto.Sentence, ctx *collectorContext,
) {
	if reply.Map[property] == "" {
		return
	}

	value, err := strconv.ParseFloat(reply.Map[property], 64)
	if err != nil {
		log.WithFields(log.Fields{
			"property":  property,
			"interface": iface,
			"device":    ctx.device.Name,
			"error":     err,
		}).Error("error parsing interface metric value")

		return
	}

	desc := c.descriptions[property]
	ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value,
		ctx.device.Name, ctx.device.Address,
		iface, reply.Map["channel"])
}

func (c *wlanIFCollector) collectMetricForFreq(iface string, re *proto.Sentence, ctx *collectorContext) {
	channel := re.Map["channel"]

	for idx, part := range strings.Split(channel, "+") {
		freq, _, found := strings.Cut(part, "/")
		if !found {
			freq = part
		}

		if freq == "" {
			continue
		}

		value, err := strconv.ParseFloat(freq, 64)
		if err != nil {
			log.WithFields(log.Fields{
				"property":  freq,
				"interface": iface,
				"device":    ctx.device.Name,
				"error":     err,
			}).Error("error parsing frequency metric value")

			return
		}

		ctx.ch <- prometheus.MustNewConstMetric(c.frequency, prometheus.GaugeValue,
			value, ctx.device.Name, ctx.device.Address, iface, strconv.Itoa(idx+1))
	}
}
