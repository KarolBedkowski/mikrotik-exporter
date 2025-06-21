package collectors

// TODO: skipping disabled; check

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"mikrotik-exporter/routeros/proto"
)

func init() {
	registerCollector("wlanif", newWlanIFCollector, "retrieves WLAN interface metrics")
}

type wlanIFCollector struct {
	frequencyDesc *prometheus.Desc
	metrics       PropertyMetricList
	channelDesc   *prometheus.Desc
}

func newWlanIFCollector() RouterOSCollector {
	const prefix = "wlan_interface"

	labelNames := []string{"name", "address", "interface"}

	return &wlanIFCollector{
		metrics: PropertyMetricList{
			NewPropertyGaugeMetric(prefix, "registered-clients", labelNames).Build(),
			NewPropertyGaugeMetric(prefix, "noise-floor", labelNames).Build(),
			NewPropertyGaugeMetric(prefix, "overall-tx-ccq", labelNames).Build(),
		},
		frequencyDesc: description(prefix, "frequency", "WiFi frequency",
			[]string{"name", "address", "interface", "freqidx"}),
		channelDesc: description(prefix, "channel", "WiFi channel",
			[]string{"name", "address", "interface", "channel"}),
	}
}

func (c *wlanIFCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.frequencyDesc
	c.metrics.Describe(ch)
	ch <- c.channelDesc
}

func (c *wlanIFCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/interface/wireless/print", "=.proplist=name,disabled,frequency")
	if err != nil {
		return fmt.Errorf("fetch wireless error: %w", err)
	}

	for _, re := range reply.Re {
		// skip disabled interfaces without frequency; if there is frequeny - interface is managed by capsman
		if re.Map["disabled"] == "true" && re.Map["frequency"] == "" {
			continue
		}

		if err := c.collectForInterface(re.Map["name"], ctx); err != nil {
			return err
		}
	}

	return nil
}

func (c *wlanIFCollector) collectForInterface(iface string, ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/interface/wireless/monitor", "=numbers="+iface, "=once=",
		"=.proplist=registered-clients,noise-floor,overall-tx-ccq,channel")
	if err != nil {
		return fmt.Errorf("fetch wireless monitor for %s error: %w", iface, err)
	}

	if len(reply.Re) == 0 {
		return nil
	}

	re := reply.Re[0]

	lctx := ctx.withLabels(iface)

	if err := c.metrics.Collect(re, &lctx); err != nil {
		return fmt.Errorf("collect %s error: %w", iface, err)
	}

	return c.collectMetricForFreq(iface, re, ctx)
}

func (c *wlanIFCollector) collectMetricForFreq(iface string, re *proto.Sentence, ctx *CollectorContext) error {
	channel := re.Map["channel"]

	// TODO: skip without channel?
	ctx.ch <- prometheus.MustNewConstMetric(c.channelDesc, prometheus.GaugeValue,
		1, ctx.device.Name, ctx.device.Address, iface, channel)

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
			return fmt.Errorf("collect channel for %s parse %v error: %w", iface, freq, err)
		}

		ctx.ch <- prometheus.MustNewConstMetric(c.frequencyDesc, prometheus.GaugeValue,
			value, ctx.device.Name, ctx.device.Address, iface, strconv.Itoa(idx+1))
	}

	return nil
}
