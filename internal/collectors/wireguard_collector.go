package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("wireguard", newWireguardCollector,
		"retrieves wireguard peers metrics")
}

type wireguardCollector struct {
	peers      metrics.PropertyMetricList
	wg         metrics.PropertyMetricList
	peersStats *prometheus.Desc
}

func newWireguardCollector() RouterOSCollector {
	const prefix = "wireguard"

	peerLabelNames := []string{"public_key", metrics.LabelComment, "disabled"}
	wgLabelNames := []string{"public_key", "wg_name", metrics.LabelComment}

	return &wireguardCollector{
		peers: metrics.PropertyMetricList{
			metrics.NewPropertyGaugeMetric(prefix, "last-handshake", peerLabelNames...).
				WithConverter(convert.MetricFromDuration).
				Build(),
			metrics.NewPropertyCounterMetric(prefix, "rx", peerLabelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "tx", peerLabelNames...).Build(),
		},
		wg: metrics.PropertyMetricList{
			metrics.NewPropertyGaugeMetric(prefix, "running", wgLabelNames...).
				WithConverter(convert.MetricFromBool).
				Build(),
		},

		peersStats: metrics.Description(prefix, "peers_status", "wg peer statistics",
			metrics.LabelDevName, metrics.LabelDevAddress, "status"),
	}
}

func (c *wireguardCollector) Describe(ch chan<- *prometheus.Desc) {
	c.peers.Describe(ch)
	c.wg.Describe(ch)
	ch <- c.peersStats
}

func (c *wireguardCollector) Collect(ctx *metrics.CollectorContext) error {
	if ctx.Device.FirmwareVersion.Major < 7 { //nolint:mnd
		return NotSupportedError("dns_adlist")
	}

	return multierror.Append(nil,
		c.collectWG(ctx),
		c.collectWGPeers(ctx),
	).ErrorOrNil()
}

func (c *wireguardCollector) collectWGPeers(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/interface/wireguard/peers/print",
		"=.proplist=comment,public-key,comment,disabled,last-handshake,rx,tx,current-endpoint-address")
	if err != nil {
		return fmt.Errorf("fetch wireguard peers stats error: %w", err)
	}

	connected := 0

	for _, re := range reply.Re {
		if re.Map["public-key"] != "" && re.Map["current-endpoint-address"] != "" {
			connected++
		}
	}

	ctx.Ch <- prometheus.MustNewConstMetric(c.peersStats, prometheus.GaugeValue, float64(connected),
		ctx.Device.Name, ctx.Device.Address, "connected")
	ctx.Ch <- prometheus.MustNewConstMetric(c.peersStats, prometheus.GaugeValue, float64(len(reply.Re)-connected),
		ctx.Device.Name, ctx.Device.Address, "waiting")

	// do not load entries if not configured
	if !ctx.FeatureCfg.BoolValue("details", false) {
		return nil
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		pubKey := re.Map["public-key"]
		if pubKey == "" {
			continue
		}

		lctx := ctx.WithLabels(pubKey, re.Map["comment"], re.Map["disabled"])

		if err := c.peers.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs,
				fmt.Errorf("collect wireguard %v error: %w", re, err))
		}
	}

	return errs.ErrorOrNil()
}

func (c *wireguardCollector) collectWG(ctx *metrics.CollectorContext) error {
	var errs *multierror.Error

	reply, err := ctx.Client.Run("/interface/wireguard/print",
		"?disabled=false",
		"=.proplist=comment,public-key,comment,disabled,running,name")
	if err != nil {
		errs = multierror.Append(fmt.Errorf("fetch wireguard status error: %w", err))

		return errs.ErrorOrNil()
	}

	for _, re := range reply.Re {
		lctx := ctx.WithLabelsFromMap(re.Map, "public-key", "name", "comment")

		if err := c.wg.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs,
				fmt.Errorf("collect wireguard %v error: %w", re, err))
		}
	}

	return errs.ErrorOrNil()
}
