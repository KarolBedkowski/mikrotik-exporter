package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("ipsec", newIpsecCollector, "retrieves IPsec metrics")
}

type ipsecCollector struct {
	metrics     PropertyMetricList
	activePeers PropertyMetricList
}

func newIpsecCollector() RouterOSCollector {
	const (
		prefix      = "ipsec"
		prefixPeers = "ipsec_active_peers"
	)

	labels := []string{"src_address", "dst_address", "comment"}
	labelsPeers := []string{"src_address", "dst_address", "comment", "side"}

	return &ipsecCollector{
		metrics: PropertyMetricList{
			NewPropertyGaugeMetric(prefix, "ph2-state", labels).WithConverter(metricFromState).Build(),
			NewPropertyGaugeMetric(prefix, "invalid", labels).WithConverter(metricFromBool).Build(),
			NewPropertyGaugeMetric(prefix, "active", labels).WithConverter(metricFromBool).Build(),
		},
		activePeers: PropertyMetricList{
			NewPropertyGaugeMetric(prefixPeers, "rx-bytes", labelsPeers).Build(),
			NewPropertyGaugeMetric(prefixPeers, "tx-bytes", labelsPeers).Build(),
			NewPropertyGaugeMetric(prefixPeers, "rx-packets", labelsPeers).Build(),
			NewPropertyGaugeMetric(prefixPeers, "tx-packets", labelsPeers).Build(),
			NewPropertyGaugeMetric(prefixPeers, "state", labelsPeers).WithConverter(metricFromState).
				WithName("established").Build(),
			NewPropertyGaugeMetric(prefixPeers, "uptime", labelsPeers).WithConverter(metricFromDuration).
				WithName("uptime_seconds").Build(),
			NewPropertyGaugeMetric(prefixPeers, "last-seen", labelsPeers).WithConverter(metricFromDuration).
				WithName("last_seen_seconds").Build(),
			NewPropertyGaugeMetric(prefixPeers, "responder", labelsPeers).WithConverter(metricFromBool).
				Build(),
		},
	}
}

func (c *ipsecCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
	c.activePeers.Describe(ch)
}

func (c *ipsecCollector) Collect(ctx *CollectorContext) error {
	return multierror.Append(nil,
		c.collectPolicy(ctx),
		c.collectActivePeers(ctx),
	).ErrorOrNil()
}

func (c *ipsecCollector) collectPolicy(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/ip/ipsec/policy/print", "?disabled=false", "?dynamic=false",
		"=.proplist=src-address,dst-address,comment,ph2-state,invalid,active")
	if err != nil {
		return fmt.Errorf("fetch ipsec policy error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.withLabels(re.Map["comment"])
		if err := c.metrics.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect policy error %w", err))
		}
	}

	return errs.ErrorOrNil()
}

func (c *ipsecCollector) collectActivePeers(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/ip/ipsec/active-peers/print",
		"=.proplist=src-address,dst-address,comment,side,rx-bytes,tx-bytes,"+
			"rx-packets,tx-packets,state,uptime,last-seen,responder")
	if err != nil {
		return fmt.Errorf("fetch ipsec active peers error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.withLabelsFromMap(re.Map, "comment", "side")
		if err := c.activePeers.Collect(re, &lctx); err != nil {
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
