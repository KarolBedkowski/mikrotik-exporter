package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("capsman", newCapsmanCollector, "retrieves CapsMan station metrics")
}

type capsmanCollector struct {
	metrics               PropertyMetricList
	interfaces            PropertyMetricList
	interfacesStatus      PropertyMetric
	radiosProvisionedDesc PropertyMetric
}

func newCapsmanCollector() RouterOSCollector {
	const (
		prefix      = "capsman_station"
		prefixIface = "capsman_interface"
	)

	labelNames := []string{LabelInterface, "mac_address", "ssid", "eap_identity", LabelComment}
	radioLabelNames := []string{LabelInterface, "radio_mac", "remote_cap_identity", "remote_cap_name"}
	ifaceLabelNames := []string{LabelInterface, "mac_address", "configuration", "master_interface"}
	ifaceStatusLabelNames := []string{
		LabelInterface, "mac_address", "configuration", "master_interface", "current_state",
	}

	return &capsmanCollector{
		metrics: PropertyMetricList{
			NewPropertyCounterMetric(prefix, "uptime", labelNames...).
				WithConverter(metricFromDuration).
				WithName("uptime_seconds").
				Build(),
			NewPropertyGaugeMetric(prefix, "tx-signal", labelNames...).Build(),
			NewPropertyGaugeMetric(prefix, "rx-signal", labelNames...).Build(),
			NewPropertyRxTxMetric(prefix, "packets", labelNames...).Build(),
			NewPropertyRxTxMetric(prefix, "bytes", labelNames...).Build(),
		},
		interfaces: PropertyMetricList{
			NewPropertyGaugeMetric(prefixIface, "current-authorized-clients", ifaceLabelNames...).Build(),
			NewPropertyGaugeMetric(prefixIface, "current-registered-clients", ifaceLabelNames...).Build(),
			NewPropertyGaugeMetric(prefixIface, "running", ifaceLabelNames...).
				WithConverter(metricFromBool).
				Build(),
			NewPropertyGaugeMetric(prefixIface, "master", ifaceLabelNames...).
				WithConverter(metricFromBool).
				Build(),
			NewPropertyGaugeMetric(prefixIface, "inactive", ifaceLabelNames...).
				WithConverter(metricFromBool).
				Build(),
		},
		interfacesStatus: NewPropertyConstMetric(prefixIface, "current-state", ifaceStatusLabelNames...).
			WithName("state").
			Build(),
		radiosProvisionedDesc: NewPropertyGaugeMetric("capsman", "provisioned", radioLabelNames...).
			WithName("radio_provisioned").
			WithHelp("Status of provision remote radios").
			WithConverter(metricFromBool).
			Build(),
	}
}

func (c *capsmanCollector) Describe(ch chan<- *prometheus.Desc) {
	c.radiosProvisionedDesc.Describe(ch)
	c.interfaces.Describe(ch)
	c.interfacesStatus.Describe(ch)
	c.metrics.Describe(ch)
}

func (c *capsmanCollector) Collect(ctx *CollectorContext) error {
	return multierror.Append(nil,
		c.collectRegistrations(ctx),
		c.collectInterfaces(ctx),
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
		lctx := ctx.withLabelsFromMap(re.Map, "interface", "mac-address", "ssid", "eap-identity", "comment")

		if err := c.metrics.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect registrations error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}

func (c *capsmanCollector) collectInterfaces(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/caps-man/interface/print",
		"?disabled=false",
		"=.proplist=name,mac-address,configuration,current-state,master-interface,"+
			"current-authorized-clients,current-registered-clients,running,master,inactive,disabled")
	if err != nil {
		return fmt.Errorf("fetch capsman interfaces error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.withLabelsFromMap(re.Map, "name", "mac-address", "configuration", "master-interface")

		if err := c.interfaces.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect interfaces error: %w", err))
		}

		lctx = lctx.appendLabelsFromMap(re.Map, "current-state")
		if err := c.interfacesStatus.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect interfaces error: %w", err))
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
		lctx := ctx.withLabelsFromMap(re.Map, "interface", "radio-mac", "remote-cap-identity", "remote-cap-name")

		if err := c.radiosProvisionedDesc.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect provisions error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}
