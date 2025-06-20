package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("wireguard", newWireguardCollector,
		"retrieves wireguard peers metrics")
}

type wireguardCollector struct {
	metrics PropertyMetricList
	wg      PropertyMetricList
}

func newWireguardCollector() RouterOSCollector {
	const prefix = "wireguard"

	labelNames := []string{"name", "address", "public_key", "comment", "disabled"}
	wgLabelNames := []string{"name", "address", "public_key", "wg_name", "comment"}

	return &wireguardCollector{
		metrics: PropertyMetricList{
			NewPropertyGaugeMetric(prefix, "last-handshake", labelNames).WithConverter(metricFromDuration).Build(),
			NewPropertyCounterMetric(prefix, "rx", labelNames).Build(),
			NewPropertyCounterMetric(prefix, "tx", labelNames).Build(),
		},
		wg: PropertyMetricList{
			NewPropertyGaugeMetric(prefix, "running", wgLabelNames).WithConverter(metricFromBool).Build(),
		},
	}
}

func (c *wireguardCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *wireguardCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/interface/wireguard/peers/print",
		"=.proplist=comment,public-key,comment,disabled,last-handshake,rx,tx")
	if err != nil {
		return fmt.Errorf("fetch wireguard peers stats error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		pubKey := re.Map["public-key"]
		if pubKey == "" {
			continue
		}

		lctx := ctx.withLabels(pubKey, re.Map["comment"], re.Map["disabled"])

		if err := c.metrics.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs,
				fmt.Errorf("collect wireguard %v error: %w", re, err))
		}
	}

	reply, err = ctx.client.Run("/interface/wireguard/print",
		"=.proplist=comment,public-key,comment,disabled,running,name")
	if err != nil {
		errs = multierror.Append(fmt.Errorf("fetch wireguard status error: %w", err))

		return errs.ErrorOrNil()
	}

	for _, re := range reply.Re {
		lctx := ctx.withLabelsFromMap(re.Map, "public-key", "name", "comment")

		if err := c.wg.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs,
				fmt.Errorf("collect wireguard %v error: %w", re, err))
		}
	}

	return errs.ErrorOrNil()
}
