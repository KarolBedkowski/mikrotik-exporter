package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("capsman", newCapsmanCollector, "retrieves CapsMan station metrics")
}

type capsmanCollector struct {
	// metric for each registered station.
	stations metrics.PropertyMetric
	// metrics for interfaces.
	interfaces metrics.PropertyMetric
	// status each interface.
	interfacesStatus metrics.PropertyMetric
	// status of provision remote radio.
	radiosProvisionedDesc metrics.PropertyMetric
}

func newCapsmanCollector() RouterOSCollector {
	const (
		prefixStation = "capsman_station"
		prefixIface   = "capsman_interface"
	)

	labelNames := []string{metrics.LabelInterface, "mac_address", "ssid", "eap_identity", metrics.LabelComment}
	radioLabelNames := []string{metrics.LabelInterface, "radio_mac", "remote_cap_identity", "remote_cap_name"}
	ifaceLabelNames := []string{metrics.LabelInterface, "mac_address", "configuration", "master_interface"}
	ifaceStatusLabelNames := []string{
		metrics.LabelInterface, "mac_address", "configuration", "master_interface", "current_state",
	}

	return &capsmanCollector{
		stations: metrics.PropertyMetricList{
			metrics.NewPropertyCounterMetric(prefixStation, "uptime", labelNames...).
				WithConverter(convert.MetricFromDuration).
				WithName("uptime_seconds").
				Build(),
			metrics.NewPropertyGaugeMetric(prefixStation, "tx-signal", labelNames...).Build(),
			metrics.NewPropertyGaugeMetric(prefixStation, "rx-signal", labelNames...).Build(),
			metrics.NewPropertyRxTxMetric(prefixStation, "packets", labelNames...).Build(),
			metrics.NewPropertyRxTxMetric(prefixStation, "bytes", labelNames...).Build(),
		},
		interfaces: metrics.PropertyMetricList{
			metrics.NewPropertyGaugeMetric(prefixIface, "current-authorized-clients", ifaceLabelNames...).Build(),
			metrics.NewPropertyGaugeMetric(prefixIface, "current-registered-clients", ifaceLabelNames...).Build(),
			metrics.NewPropertyGaugeMetric(prefixIface, "running", ifaceLabelNames...).
				WithConverter(convert.MetricFromBool).
				Build(),
			metrics.NewPropertyGaugeMetric(prefixIface, "master", ifaceLabelNames...).
				WithConverter(convert.MetricFromBool).
				Build(),
			metrics.NewPropertyGaugeMetric(prefixIface, "inactive", ifaceLabelNames...).
				WithConverter(convert.MetricFromBool).
				Build(),
		},
		interfacesStatus: metrics.NewPropertyConstMetric(prefixIface, "current-state", ifaceStatusLabelNames...).
			WithName("state").
			Build(),
		radiosProvisionedDesc: metrics.NewPropertyGaugeMetric("capsman", "provisioned", radioLabelNames...).
			WithName("radio_provisioned").
			WithHelp("Status of provision remote radios").
			WithConverter(convert.MetricFromBool).
			Build(),
	}
}

func (c *capsmanCollector) Describe(ch chan<- *prometheus.Desc) {
	c.radiosProvisionedDesc.Describe(ch)
	c.interfaces.Describe(ch)
	c.interfacesStatus.Describe(ch)
	c.stations.Describe(ch)
}

func (c *capsmanCollector) Collect(ctx *metrics.CollectorContext) error {
	return multierror.Append(nil,
		c.collectRegistrations(ctx),
		c.collectInterfaces(ctx),
		c.collectRadiosProvisioned(ctx),
	).ErrorOrNil()
}

func (c *capsmanCollector) collectRegistrations(ctx *metrics.CollectorContext) error {
	// do not load stations details when not configured
	if !ctx.FeatureCfg.BoolValue("stations", false) {
		return nil
	}

	reply, err := ctx.Client.Run("/caps-man/registration-table/print",
		"=.proplist=interface,mac-address,ssid,uptime,tx-signal,rx-signal,packets,bytes,eap-identity,comment")
	if err != nil {
		return fmt.Errorf("fetch capsman reg error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.WithLabelsFromMap(re.Map, "interface", "mac-address", "ssid", "eap-identity", "comment")

		if err := c.stations.Collect(re.Map, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect registrations error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}

func (c *capsmanCollector) collectInterfaces(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/caps-man/interface/print",
		"?disabled=false",
		"=.proplist=name,mac-address,configuration,current-state,master-interface,"+
			"current-authorized-clients,current-registered-clients,running,master,inactive,disabled")
	if err != nil {
		return fmt.Errorf("fetch capsman interfaces error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.WithLabelsFromMap(re.Map, "name", "mac-address", "configuration", "master-interface")

		if err := c.interfaces.Collect(re.Map, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect interfaces error: %w", err))
		}

		lctx.AppendLabelsFromMap(re.Map, "current-state")

		if err := c.interfacesStatus.Collect(re.Map, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect interfaces error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}

func (c *capsmanCollector) collectRadiosProvisioned(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/caps-man/radio/print",
		"=.proplist=interface,radio-mac,remote-cap-identity,remote-cap-name,provisioned")
	if err != nil {
		return fmt.Errorf("fetch capsman radio error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.WithLabelsFromMap(re.Map, "interface", "radio-mac", "remote-cap-identity", "remote-cap-name")

		if err := c.radiosProvisionedDesc.Collect(re.Map, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect provisions error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}
