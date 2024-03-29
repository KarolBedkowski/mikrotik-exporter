package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("capsman", newCapsmanCollector,
		"retrieves CapsMan station metrics")
}

type capsmanCollector struct {
	metrics PropertyMetricList

	radiosProvisionedDesc PropertyMetric
}

func newCapsmanCollector() RouterOSCollector {
	const prefix = "capsman_station"

	labelNames := []string{"name", "address", "interface", "mac_address", "ssid", "eap_identity", "comment"}
	radioLabelNames := []string{"name", "address", "interface", "radio_mac", "remote_cap_identity", "remote_cap_name"}

	collector := &capsmanCollector{
		metrics: PropertyMetricList{
			NewPropertyCounterMetric(prefix, "uptime", labelNames).WithConverter(metricFromDuration).
				WithName("uptime_seconds").Build(),
			NewPropertyGaugeMetric(prefix, "tx-signal", labelNames).Build(),
			NewPropertyGaugeMetric(prefix, "rx-signal", labelNames).Build(),
			NewPropertyRxTxMetric(prefix, "packets", labelNames).Build(),
			NewPropertyRxTxMetric(prefix, "bytes", labelNames).Build(),
		},
		radiosProvisionedDesc: NewPropertyGaugeMetric("capsman", "provisioned", radioLabelNames).
			WithName("radio_provisioned").WithHelp("Status of provision remote radios").
			WithConverter(metricFromBool).
			Build(),
	}

	return collector
}

func (c *capsmanCollector) Describe(ch chan<- *prometheus.Desc) {
	c.radiosProvisionedDesc.Describe(ch)
	c.metrics.Describe(ch)
}

func (c *capsmanCollector) Collect(ctx *CollectorContext) error {
	return multierror.Append(nil,
		c.collectRegistrations(ctx),
		c.collectRadiosProvisioned(ctx),
	).ErrorOrNil()
}

func (c *capsmanCollector) collectRegistrations(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/caps-man/registration-table/print",
		"=.proplist=interface,mac-address,ssid,uptime,tx-signal,rx-signal,packets,bytes,eap-identity,comment")
	if err != nil {
		return fmt.Errorf("fetch capsman reg error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.withLabels(re.Map["interface"], re.Map["mac-address"], re.Map["ssid"],
			re.Map["eap-identity"], re.Map["comment"])

		if err := c.metrics.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect registrations error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}

func (c *capsmanCollector) collectRadiosProvisioned(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/caps-man/radio/print",
		"=.proplist=interface,radio-mac,remote-cap-identity,remote-cap-name,provisioned")
	if err != nil {
		return fmt.Errorf("fetch capsman radio error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.withLabels(re.Map["interface"], re.Map["radio-mac"], re.Map["remote-cap-identity"],
			re.Map["remote-cap-name"])

		if err := c.radiosProvisionedDesc.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect provisions error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}
