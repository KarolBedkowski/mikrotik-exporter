package collector

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

// TODO: need verify

func init() {
	registerCollector("ipsec", newIpsecCollector)
}

type ipsecCollector struct {
	metrics propertyMetricList
}

func newIpsecCollector() routerOSCollector {
	const prefix = "ipsec"

	labels := []string{"src_address", "dst_address", "comment"}
	c := &ipsecCollector{
		metrics: propertyMetricList{
			newPropertyGaugeMetric(prefix, "ph2-state", labels).withConverter(convertPH2State).build(),
			newPropertyGaugeMetric(prefix, "invalid", labels).withConverter(convertFromBool).build(),
			newPropertyGaugeMetric(prefix, "active", labels).withConverter(convertFromBool).build(),
		},
	}

	return c
}

func (c *ipsecCollector) describe(ch chan<- *prometheus.Desc) {
	c.metrics.describe(ch)
}

func (c *ipsecCollector) collect(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/ip/ipsec/policy/print", "?disabled=false", "?dynamic=false",
		"=.proplist=src-address,dst-address,ph2-state,invalid,active,comment")
	if err != nil {
		return fmt.Errorf("fetch ipsec policy error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		if err := c.metrics.collect(re, ctx); err != nil {
			errs = multierror.Append(errs, err)
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
