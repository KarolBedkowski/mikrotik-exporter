package collector

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("ipsec", newIpsecCollector)
}

type ipsecCollector struct {
	metrics     propertyMetricList
	activePeers propertyMetricList
}

func newIpsecCollector() routerOSCollector {
	const (
		prefix      = "ipsec"
		prefixPeers = "ipsec_active_peers"
	)

	labels := []string{"src_address", "dst_address", "comment"}
	labelsPeers := []string{"src_address", "dst_address", "comment", "side"}

	return &ipsecCollector{
		metrics: propertyMetricList{
			newPropertyGaugeMetric(prefix, "ph2-state", labels).withConverter(convertPH2State).build(),
			newPropertyGaugeMetric(prefix, "invalid", labels).withConverter(convertFromBool).build(),
			newPropertyGaugeMetric(prefix, "active", labels).withConverter(convertFromBool).build(),
		},
		activePeers: propertyMetricList{
			newPropertyGaugeMetric(prefixPeers, "rx-bytes", labelsPeers).build(),
			newPropertyGaugeMetric(prefixPeers, "tx-bytes", labelsPeers).build(),
			newPropertyGaugeMetric(prefixPeers, "rx-packets", labelsPeers).build(),
			newPropertyGaugeMetric(prefixPeers, "tx-packets", labelsPeers).build(),
			newPropertyGaugeMetric(prefixPeers, "state", labelsPeers).withConverter(convertFromState).
				withName("established").build(),
			newPropertyGaugeMetric(prefixPeers, "uptime", labelsPeers).withConverter(parseDuration).
				withName("uptime_seconds").build(),
			newPropertyGaugeMetric(prefixPeers, "last-seen", labelsPeers).withConverter(parseDuration).
				withName("last_seen_seconds").build(),
			newPropertyGaugeMetric(prefixPeers, "responder", labelsPeers).withConverter(convertFromBool).
				build(),
		},
	}
}

func (c *ipsecCollector) describe(ch chan<- *prometheus.Desc) {
	c.metrics.describe(ch)
	c.activePeers.describe(ch)
}

func (c *ipsecCollector) collect(ctx *collectorContext) error {
	var errs *multierror.Error

	if err := c.collectPolicy(ctx); err != nil {
		errs = multierror.Append(errs, err)
	}

	if err := c.collectActivePeers(ctx); err != nil {
		errs = multierror.Append(errs, err)
	}

	return errs.ErrorOrNil()
}

func (c *ipsecCollector) collectPolicy(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/ip/ipsec/policy/print", "?disabled=false", "?dynamic=false",
		"=.proplist=src-address,dst-address,comment,ph2-state,invalid,active")
	if err != nil {
		return fmt.Errorf("fetch ipsec policy error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		ctx = ctx.withLabels(re.Map["comment"])
		if err := c.metrics.collect(re, ctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect policy error %w", err))
		}
	}

	return errs.ErrorOrNil()
}

func (c *ipsecCollector) collectActivePeers(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/ip/ipsec/active-peers/print",
		"=.proplist=src-address,dst-address,comment,side,rx-bytes,tx-bytes,"+
			"rx-packets,tx-packets,state,uptime,last-seen,responder")
	if err != nil {
		return fmt.Errorf("fetch ipsec active peers error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		ctx = ctx.withLabels(re.Map["comment"], re.Map["side"])
		if err := c.activePeers.collect(re, ctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect active peers error %w", err))
		}
	}

	return errs.ErrorOrNil()
}

func convertPH2State(value string) (float64, error) {
	if value == "established" {
		return 1.0, nil
	}

	return 0.0, nil
}

func convertFromState(value string) (float64, error) {
	if value == "established" {
		return 1.0, nil
	}

	return 0.0, nil
}
