package collector

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("capsman", newCapsmanCollector)
}

type capsmanCollector struct {
	metrics propertyMetricList

	radiosProvisionedDesc propertyMetricCollector
}

func newCapsmanCollector() routerOSCollector {
	const prefix = "capsman_station"

	labelNames := []string{"name", "address", "interface", "mac_address", "ssid", "eap_identity", "comment"}
	radioLabelNames := []string{"name", "address", "interface", "radio_mac", "remote_cap_identity", "remote_cap_name"}

	collector := &capsmanCollector{
		metrics: propertyMetricList{
			newPropertyCounterMetric(prefix, "uptime", labelNames).withConverter(parseDuration).
				withName("uptime_seconds").build(),
			newPropertyGaugeMetric(prefix, "tx-signal", labelNames).build(),
			newPropertyGaugeMetric(prefix, "rx-signal", labelNames).build(),
			newPropertyRxTxMetric(prefix, "packets", labelNames).build(),
			newPropertyRxTxMetric(prefix, "bytes", labelNames).build(),
		},

		radiosProvisionedDesc: newPropertyGaugeMetric("capsman", "provisioned", radioLabelNames).
			withName("radio_provisioned").withHelp("Status of provision remote radios").
			withConverter(convertFromBool).
			build(),
	}

	return collector
}

func (c *capsmanCollector) describe(ch chan<- *prometheus.Desc) {
	c.radiosProvisionedDesc.describe(ch)
	c.metrics.describe(ch)
}

func (c *capsmanCollector) collect(ctx *collectorContext) error {
	var errs *multierror.Error

	if err := c.collectRegistrations(ctx); err != nil {
		errs = multierror.Append(errs, err)
	}

	if err := c.collectRadiosProvisioned(ctx); err != nil {
		errs = multierror.Append(errs, err)
	}

	return errs.ErrorOrNil()
}

func (c *capsmanCollector) collectRegistrations(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/caps-man/registration-table/print",
		"=.proplist=interface,mac-address,ssid,uptime,tx-signal,rx-signal,packets,bytes,eap-identity,comment")
	if err != nil {
		return fmt.Errorf("fetch capsman reg error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		ctx = ctx.withLabels(re.Map["interface"], re.Map["mac-address"], re.Map["ssid"],
			re.Map["eap-identity"], re.Map["comment"])

		if err := c.metrics.collect(re, ctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect registrations error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}

func (c *capsmanCollector) collectRadiosProvisioned(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/caps-man/radio/print",
		"=.proplist=interface,radio-mac,remote-cap-identity,remote-cap-name,provisioned")
	if err != nil {
		return fmt.Errorf("fetch capsman radio error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		ctx = ctx.withLabels(re.Map["interface"], re.Map["radio-mac"], re.Map["remote-cap-identity"],
			re.Map["remote-cap-name"])

		if err := c.radiosProvisionedDesc.collect(re, ctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect provisions error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}
