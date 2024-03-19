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
	metrics []propertyMetricCollector

	frequencyDesc *prometheus.Desc
}

func newWlanIFCollector() routerOSCollector {
	const prefix = "wlan_interface"

	labelNames := []string{"name", "address", "interface", "channel"}

	collector := &wlanIFCollector{
		metrics: []propertyMetricCollector{
			newPropertyGaugeMetric(prefix, "registered-clients", labelNames).build(),
			newPropertyGaugeMetric(prefix, "noise-floor", labelNames).build(),
			newPropertyGaugeMetric(prefix, "overall-tx-ccq", labelNames).build(),
		},
		frequencyDesc: description(prefix, "frequency", "WiFi frequency",
			[]string{"name", "address", "interface", "freqidx"}),
	}

	return collector
}

func (c *wlanIFCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.frequencyDesc

	for _, c := range c.metrics {
		c.describe(ch)
	}
}

func (c *wlanIFCollector) collect(ctx *collectorContext) error {
	names, err := c.fetchInterfaceNames(ctx)
	if err != nil {
		return err
	}

	for _, n := range names {
		if err := c.collectForInterface(n, ctx); err != nil {
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
	reply, err := ctx.client.Run("/interface/wireless/monitor", "=numbers="+iface, "=once=",
		"=.proplist=registered-clients,noise-floor,overall-tx-ccq,channel")
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

	re := reply.Re[0]

	ctx = ctx.withLabels(iface, re.Map["channel"])

	for _, c := range c.metrics {
		_ = c.collect(re, ctx)
	}

	c.collectMetricForFreq(iface, re, ctx)

	return nil
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

		ctx.ch <- prometheus.MustNewConstMetric(c.frequencyDesc, prometheus.GaugeValue,
			value, ctx.device.Name, ctx.device.Address, iface, strconv.Itoa(idx+1))
	}
}
