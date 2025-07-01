package collectors

// TODO: skipping disabled; check

import (
	"fmt"
	"strconv"
	"strings"

	"mikrotik-exporter/internal/metrics"
	"mikrotik-exporter/routeros/proto"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("wlanif", newWlanIFCollector, "retrieves WLAN interface metrics")
}

type wlanIFCollector struct {
	frequencyDesc *prometheus.Desc
	metrics       metrics.PropertyMetricList
	channelDesc   *prometheus.Desc
}

func newWlanIFCollector() RouterOSCollector {
	const prefix = "wlan_interface"

	return &wlanIFCollector{
		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyGaugeMetric(prefix, "registered-clients", metrics.LabelInterface).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "noise-floor", metrics.LabelInterface).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "overall-tx-ccq", metrics.LabelInterface).Build(),
		},
		frequencyDesc: metrics.Description(prefix, "frequency", "WiFi frequency",
			metrics.LabelDevName, metrics.LabelDevAddress, metrics.LabelInterface, "freqidx"),
		channelDesc: metrics.Description(prefix, "channel", "WiFi channel",
			metrics.LabelDevName, metrics.LabelDevAddress, metrics.LabelInterface, "channel"),
	}
}

func (c *wlanIFCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.frequencyDesc
	c.metrics.Describe(ch)
	ch <- c.channelDesc
}

func (c *wlanIFCollector) Collect(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/interface/wireless/print", "=.proplist=name,disabled,frequency")
	if err != nil {
		return fmt.Errorf("fetch wireless error: %w", err)
	}

	for _, re := range reply.Re {
		// skip disabled interfaces without frequency; if there is frequency - interface is managed by capsman
		if re.Map["disabled"] == "true" && re.Map["frequency"] == "" {
			continue
		}

		if err := c.collectForInterface(re.Map["name"], ctx); err != nil {
			return err
		}
	}

	return nil
}

func (c *wlanIFCollector) collectForInterface(iface string, ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/interface/wireless/monitor", "=numbers="+iface, "=once=",
		"=.proplist=registered-clients,noise-floor,overall-tx-ccq,channel")
	if err != nil {
		return fmt.Errorf("fetch wireless monitor for %s error: %w", iface, err)
	}

	if len(reply.Re) == 0 {
		return nil
	}

	re := reply.Re[0]

	lctx := ctx.WithLabels(iface)

	if err := c.metrics.Collect(re.Map, &lctx); err != nil {
		return fmt.Errorf("collect %s error: %w", iface, err)
	}

	return c.collectMetricForFreq(iface, re, ctx)
}

func (c *wlanIFCollector) collectMetricForFreq(iface string, re *proto.Sentence, ctx *metrics.CollectorContext) error {
	channel := re.Map["channel"]

	// TODO: skip without channel?
	ctx.Ch <- prometheus.MustNewConstMetric(c.channelDesc, prometheus.GaugeValue,
		1, ctx.Device.Name, ctx.Device.Address, iface, channel)

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

		ctx.Ch <- prometheus.MustNewConstMetric(c.frequencyDesc, prometheus.GaugeValue,
			value, ctx.Device.Name, ctx.Device.Address, iface, strconv.Itoa(idx+1))
	}

	return nil
}
