package collector

import (
	"fmt"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("capsman", newCapsmanCollector)
}

type capsmanCollector struct {
	metrics []propertyMetricCollector

	radiosProvisionedDesc *prometheus.Desc
}

func newCapsmanCollector() routerOSCollector {
	const prefix = "capsman_station"

	labelNames := []string{"name", "address", "interface", "mac_address", "ssid"}
	radioLabelNames := []string{"name", "address", "interface", "radio_mac", "remote_cap_identity", "remote_cap_name"}

	collector := &capsmanCollector{
		metrics: []propertyMetricCollector{
			newPropertyCounterMetric(prefix, "uptime", labelNames).withConverter(parseDuration).build(),
			newPropertyGaugeMetric(prefix, "tx-signal", labelNames).build(),
			newPropertyGaugeMetric(prefix, "rx-signal", labelNames).build(),
			newPropertyRxTxMetric(prefix, "packets", labelNames).build(),
			newPropertyRxTxMetric(prefix, "bytes", labelNames).build(),
		},

		radiosProvisionedDesc: description("capsman", "radio_provisioned",
			"Status of provision remote radios", radioLabelNames),
	}

	return collector
}

func (c *capsmanCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.radiosProvisionedDesc

	for _, m := range c.metrics {
		m.describe(ch)
	}
}

func (c *capsmanCollector) collect(ctx *collectorContext) error {
	stats, err := c.fetch(ctx)
	if err != nil {
		return err
	}

	for _, re := range stats {
		c.collectForStat(re, ctx)
	}

	return c.collectRadiosProvisioned(ctx)
}

func (c *capsmanCollector) fetch(ctx *collectorContext) ([]*proto.Sentence, error) {
	reply, err := ctx.client.Run("/caps-man/registration-table/print",
		"=.proplist=interface,mac-address,ssid,uptime,tx-signal,rx-signal,packets,bytes")
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
	labels := []string{re.Map["interface"], re.Map["mac-address"], re.Map["ssid"]}

	for _, m := range c.metrics {
		_ = m.collect(re, ctx, labels)
	}
}

func (c *capsmanCollector) collectRadiosProvisioned(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/caps-man/radio/print",
		"=.proplist=interface,radio-mac,remote-cap-identity,remote-cap-name,provisioned")
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
