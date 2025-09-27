package collectors

import (
	"fmt"
	"strconv"
	"strings"

	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("switch", newSwitchCollector,
		"retrieves switch statistics")
}

type switchCollector struct {
	stats       *prometheus.Desc
	statsDriver metrics.PropertyMetric
}

func newSwitchCollector() RouterOSCollector {
	const (
		prefix = "switch"
	)

	labelNames := []string{"switch"}

	return &switchCollector{
		statsDriver: metrics.PropertyMetricList{
			metrics.NewPropertyCounterMetric(prefix, "driver-rx-byte", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "driver-rx-packet", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "driver-tx-byte", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "driver-tx-packet", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "rx-broadcast", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "rx-bytes", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "rx-drop", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "rx-multicast", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "rx-packet", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "tx-broadcast", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "tx-bytes", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "tx-drop", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "tx-multicast", labelNames...).Build(),
			metrics.NewPropertyCounterMetric(prefix, "tx-packet", labelNames...).Build(),
		},

		stats: metrics.Description(prefix, "stats", "switch statistics",
			metrics.LabelDevName, metrics.LabelDevAddress, "switch", "metric"),
	}
}

func (c *switchCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.stats
	c.statsDriver.Describe(ch)
}

func (c *switchCollector) Collect(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/interface/ethernet/switch/print")
	if err != nil {
		return fmt.Errorf("fetch switch stats error: %w", err)
	}

	var errs *multierror.Error

	details := ctx.FeatureCfg.BoolValue("details", false)

	for _, re := range reply.Re {
		name := re.Map["name"]

		lctx := ctx.WithLabels(name)
		if err := c.statsDriver.Collect(re.Map, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect switch %s error: %w", name, err))
		}

		// load details if configured
		if details {
			c.collectDetails(ctx, name, re.Map)
		}
	}

	return errs.ErrorOrNil()
}

func (c *switchCollector) collectDetails(ctx *metrics.CollectorContext, name string, remap map[string]string) {
	for k, v := range remap {
		if k == "name" || k == "type" || k == "invalid" || k == ".id" ||
			strings.HasPrefix(k, "mirror") || strings.HasPrefix(k, "driver-") {
			continue
		}

		// ignore non-numeric values
		if val, err := strconv.ParseInt(v, 10, 64); err == nil {
			ctx.Ch <- prometheus.MustNewConstMetric(c.stats, prometheus.CounterValue, float64(val),
				ctx.Device.Name, ctx.Device.Address, name, k)
		}
	}
}

/*
driver-rx-byte
driver-rx-packet
driver-tx-byte
driver-tx-packet

rx-1024-1518
rx-1024-max
rx-128-255
rx-1519-max
rx-256-511
rx-512-1023
rx-64
rx-65-127
rx-align-error
rx-broadcast
rx-bytes
rx-carrier-error
rx-code-error
rx-control
rx-drop
rx-error-events
rx-fcs-error
rx-fragment
rx-ip-header-checksum-error
rx-jabber
rx-length-error
rx-multicast
rx-overflow
rx-packet
rx-pause
rx-tcp-checksum-error
rx-too-long
rx-too-short
rx-udp-checksum-error
rx-unicast
rx-unknown-op

tx-1024-1518
tx-1024-max
tx-128-255
tx-1519-max
tx-256-511
tx-512-1023
tx-64
tx-65-127
tx-broadcast
tx-bytes
tx-carrier-sense-error
tx-collision
tx-control
tx-deferred
tx-drop
tx-excessive-collision
tx-excessive-deferred
tx-fcs-error
tx-fragment
tx-jabber
tx-late-collision
tx-multicast
tx-multiple-collision
tx-packet
tx-pause
tx-pause-honored
tx-single-collision
tx-too-long
tx-too-short
tx-total-collision
tx-underrun
tx-unicast


tx-all-queue-drop-byte
tx-all-queue-drop-packet
tx-queue-custom0-drop-byte
tx-queue-custom0-drop-packet
tx-queue-custom1-drop-byte
tx-queue-custom1-drop-packet
tx-queue0-byte
tx-queue0-packet
tx-queue1-byte
tx-queue1-packet
tx-queue2-byte
tx-queue2-packet
tx-queue3-byte
tx-queue3-packet
tx-queue4-byte
tx-queue4-packet
tx-queue5-byte
tx-queue5-packet
tx-queue6-byte
tx-queue6-packet
tx-queue7-byte
tx-queue7-packet

tx-rx-1024-1518
tx-rx-1024-max
tx-rx-128-255
tx-rx-1519-max
tx-rx-256-511
tx-rx-512-1023
tx-rx-64
tx-rx-65-127
*/
