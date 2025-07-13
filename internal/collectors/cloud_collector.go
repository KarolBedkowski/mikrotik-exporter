package collectors

import (
	"fmt"
	"strings"

	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/internal/metrics"

	"github.com/prometheus/client_golang/prometheus"
)

// ddns-enabled
/// vpn-status

func init() {
	registerCollector("cloud", newCloudCollector, "retrieves cloud services information")
}

type cloudCollector struct {
	ifaceStatus    metrics.PropertyMetric
	ddnsEnabled    metrics.PropertyMetric
	bthMetrics     metrics.PropertyMetric
	bthActiveUsers metrics.PropertyMetric
}

func newCloudCollector() RouterOSCollector {
	const prefix = "cloud"

	return &cloudCollector{
		// create metrics with postfix and set it to value 1 or 0 according to `status` property.
		ifaceStatus: metrics.NewPropertyConstMetric(prefix, "status", "status").Build(),

		ddnsEnabled: metrics.NewPropertyGaugeMetric(prefix, "ddns-enabled").
			WithConverter(convert.MetricFromBool).
			Build(),

		bthMetrics: metrics.PropertyMetricList{
			metrics.NewPropertyGaugeMetric(prefix, "back-to-home-vpn").
				WithName("bth_vpn_enabled").
				WithConverter(convert.MetricFromEnabled).
				Build(),
			metrics.NewPropertyGaugeMetric(prefix, "vpn-status").
				WithName("bth_vpn_running").
				WithConverter(convert.MetricFromRunning).
				WithDefault("unknown").
				Build(),
			metrics.NewPropertyGaugeMetric(prefix, "vpn-relay-ipv4-status").
				WithName("bth_vpn_relay_ipv4_reachable").
				WithConverter(bthRelayStatus).
				Build(),
			metrics.NewPropertyGaugeMetric(prefix, "vpn-relay-ipv6-status").
				WithName("bth_vpn_relay_ipv6_reachable").
				WithConverter(bthRelayStatus).
				Build(),
		},

		bthActiveUsers: metrics.NewPropertyRetMetric(prefix, "bth_users").
			WithHelp("number of active bth users").Build(),
	}
}

func (c *cloudCollector) Describe(ch chan<- *prometheus.Desc) {
	c.ifaceStatus.Describe(ch)
	c.bthMetrics.Describe(ch)
	c.bthActiveUsers.Describe(ch)
}

func (c *cloudCollector) Collect(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/ip/cloud/print")
	if err != nil {
		return fmt.Errorf("get cloud error: %w", err)
	}

	if len(reply.Re) != 1 {
		return UnexpectedResponseError{"get cloud returned more than 1 record", reply}
	}

	re := reply.Re[0]
	lctx := ctx.WithLabels(re.Map["status"])

	if err := c.ifaceStatus.Collect(re.Map, &lctx); err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	if err := c.ddnsEnabled.Collect(re.Map, ctx); err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	if ctx.Device.FirmwareVersion.Major < 7 { //nolint:mnd
		return nil
	}

	// Collect back-to-home metrics

	if err := c.bthMetrics.Collect(re.Map, ctx); err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	// count active bth users
	reply, err = ctx.Client.Run("/ip/cloud/back-to-home-user/print", "?active=true", "=count-only=")
	if err != nil {
		return fmt.Errorf("fetch active bth users error: %w", err)
	}

	if err := c.bthActiveUsers.Collect(reply.Done.Map, ctx); err != nil {
		return fmt.Errorf("parse ret %v error: %w", reply, err)
	}

	return nil
}

func bthRelayStatus(value string) (float64, error) {
	if strings.HasPrefix(value, "reachable") {
		return 1.0, nil
	}

	return 0.0, nil
}
