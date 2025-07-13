package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("ipsec", newIpsecCollector, "retrieves IPsec metrics")
}

type ipsecCollector struct {
	metrics     metrics.PropertyMetricList
	activePeers metrics.PropertyMetricList
}

func newIpsecCollector() RouterOSCollector {
	const (
		prefix      = "ipsec"
		prefixPeers = "ipsec_active_peers"
	)

	labels := []string{"src_address", "dst_address", metrics.LabelComment}
	labelsPeers := []string{"src_address", "dst_address", metrics.LabelComment, "side"}

	return &ipsecCollector{
		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyGaugeMetric(prefix, "ph2-state", labels...).WithConverter(metricFromState).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "invalid", labels...).WithConverter(convert.MetricFromBool).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "active", labels...).WithConverter(convert.MetricFromBool).Build(),
		},
		activePeers: metrics.PropertyMetricList{
			metrics.NewPropertyGaugeMetric(prefixPeers, "rx-bytes", labelsPeers...).Build(),
			metrics.NewPropertyGaugeMetric(prefixPeers, "tx-bytes", labelsPeers...).Build(),
			metrics.NewPropertyGaugeMetric(prefixPeers, "rx-packets", labelsPeers...).Build(),
			metrics.NewPropertyGaugeMetric(prefixPeers, "tx-packets", labelsPeers...).Build(),
			metrics.NewPropertyGaugeMetric(prefixPeers, "state", labelsPeers...).
				WithConverter(metricFromState).
				WithName("established").
				Build(),
			metrics.NewPropertyGaugeMetric(prefixPeers, "uptime", labelsPeers...).
				WithConverter(convert.MetricFromDuration).
				WithName("uptime_seconds").
				Build(),
			metrics.NewPropertyGaugeMetric(prefixPeers, "last-seen", labelsPeers...).
				WithConverter(convert.MetricFromDuration).
				WithName("last_seen_seconds").
				Build(),
			metrics.NewPropertyGaugeMetric(prefixPeers, "responder", labelsPeers...).
				WithConverter(convert.MetricFromBool).
				Build(),
		},
	}
}

func (c *ipsecCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
	c.activePeers.Describe(ch)
}

func (c *ipsecCollector) Collect(ctx *metrics.CollectorContext) error {
	return multierror.Append(nil,
		c.collectPolicy(ctx),
		c.collectActivePeers(ctx),
	).ErrorOrNil()
}

func (c *ipsecCollector) collectPolicy(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/ip/ipsec/policy/print",
		"?disabled=false",
		"?dynamic=false",
		"=.proplist=src-address,dst-address,comment,ph2-state,invalid,active")
	if err != nil {
		return fmt.Errorf("fetch ipsec policy error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.WithLabelsFromMap(re.Map, "src-address", "dst-address", "comment")
		if err := c.metrics.Collect(re.Map, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect policy error %w", err))
		}
	}

	return errs.ErrorOrNil()
}

func (c *ipsecCollector) collectActivePeers(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/ip/ipsec/active-peers/print",
		"=.proplist=src-address,dst-address,comment,side,rx-bytes,tx-bytes,"+
			"rx-packets,tx-packets,state,uptime,last-seen,responder")
	if err != nil {
		return fmt.Errorf("fetch ipsec active peers error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.WithLabelsFromMap(re.Map, "src-address", "dst-address", "comment", "side")
		if err := c.activePeers.Collect(re.Map, &lctx); err != nil {
			errs = multierror.Append(errs,
				fmt.Errorf("collect active peers error %w", err))
		}
	}

	return errs.ErrorOrNil()
}

func metricFromState(value string) (float64, error) {
	if value == "established" {
		return 1.0, nil
	}

	return 0.0, nil
}
