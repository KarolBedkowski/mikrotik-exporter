package collector

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("wlanif", newWlanIFCollector)
}

type wlanIFCollector struct {
	metrics propertyMetricList

	frequencyDesc *prometheus.Desc
}

func newWlanIFCollector() routerOSCollector {
	const prefix = "wlan_interface"

	labelNames := []string{"name", "address", "interface", "channel"}

	return &wlanIFCollector{
		metrics: propertyMetricList{
			newPropertyGaugeMetric(prefix, "registered-clients", labelNames).build(),
			newPropertyGaugeMetric(prefix, "noise-floor", labelNames).build(),
			newPropertyGaugeMetric(prefix, "overall-tx-ccq", labelNames).build(),
		},
		frequencyDesc: description(prefix, "frequency", "WiFi frequency",
			[]string{"name", "address", "interface", "freqidx"}),
	}
}

func (c *wlanIFCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.frequencyDesc
	c.metrics.describe(ch)
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
		return nil, fmt.Errorf("fetch wireless error: %w", err)
	}

	return extractPropertyFromReplay(reply, "name"), nil
}

func (c *wlanIFCollector) collectForInterface(iface string, ctx *collectorContext) error {
	reply, err := ctx.client.Run("/interface/wireless/monitor", "=numbers="+iface, "=once=",
		"=.proplist=registered-clients,noise-floor,overall-tx-ccq,channel")
	if err != nil {
		return fmt.Errorf("fetch wireless monitor for %s error: %w", iface, err)
	}

	if len(reply.Re) == 0 {
		return nil
	}

	re := reply.Re[0]

	ctx = ctx.withLabels(iface, re.Map["channel"])

	if err := c.metrics.collect(re, ctx); err != nil {
		return fmt.Errorf("collect %s error: %w", iface, err)
	}

	return c.collectMetricForFreq(iface, re, ctx)
}

func (c *wlanIFCollector) collectMetricForFreq(iface string, re *proto.Sentence, ctx *collectorContext) error {
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
			return fmt.Errorf("collect channel for %s perse %v error: %w", iface, freq, err)
		}

		ctx.ch <- prometheus.MustNewConstMetric(c.frequencyDesc, prometheus.GaugeValue,
			value, ctx.device.Name, ctx.device.Address, iface, strconv.Itoa(idx+1))
	}

	return nil
}
