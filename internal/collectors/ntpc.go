package collectors

import (
	"fmt"
	"strconv"

	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/internal/metrics"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("ntpc", newNTPcCollector, "retrieves ntp client metrics")
}

type ntpcCollector struct {
	metrics metrics.PropertyMetric
}

func newNTPcCollector() RouterOSCollector {
	const prefix = "ntp_client"

	return &ntpcCollector{
		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyGaugeMetric(prefix, "enabled").
				WithConverter(convert.MetricFromBool).
				Build(),
			metrics.NewPropertyGaugeMetric(prefix, "status").
				WithName("synchronized").
				WithConverter(metricFromNtpStatus).
				Build(),
			metrics.NewPropertyGaugeMetric(prefix, "system-offset").
				WithConverter(msToSecConverter).
				Build(),
		},
	}
}

func (c *ntpcCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *ntpcCollector) Collect(ctx *metrics.CollectorContext) error {
	if ctx.Device.FirmwareVersion.Major < 7 { //nolint:mnd
		return c.collectRO6(ctx)
	}

	return c.collectRO7(ctx)
}

func (c *ntpcCollector) collectRO6(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/system/ntp/client/print",
		"=.proplist=enabled,active-server,last-adjustment")
	if err != nil {
		return fmt.Errorf("fetch ntp client error: %w", err)
	}

	if len(reply.Re) == 0 {
		return nil
	}

	sent := reply.Re[0]

	if as := sent.Map["active-server"]; as != "" {
		sent.Map["status"] = "synchronized"
	} else {
		sent.Map["status"] = ""
	}

	if la := sent.Map["last-adjustment"]; la != "" {
		dur, err := convert.MetricFromDuration(la)
		if err != nil {
			return fmt.Errorf("parse last-adjustment %q error: %w", la, err)
		}

		// RO7 return data as ms, so convert seconds to ms
		sent.Map["system-offset"] = strconv.FormatFloat(dur*1000, 'f', 6, 32)
	}

	// collect metrics using context
	if err := c.metrics.Collect(sent, ctx); err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	return nil
}

func (c *ntpcCollector) collectRO7(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/system/ntp/client/print",
		"=.proplist=enabled,status,system-offset")
	if err != nil {
		return fmt.Errorf("fetch ntp client error: %w", err)
	}

	if len(reply.Re) == 0 {
		return nil
	}

	if err := c.metrics.Collect(reply.Re[0], ctx); err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	return nil
}

func msToSecConverter(input string) (float64, error) {
	value, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return 0.0, fmt.Errorf("parse %q into float error: %w", input, err)
	}

	return value / 1000.0, nil //nolint:mnd
}

func metricFromNtpStatus(input string) (float64, error) {
	if input == "synchronized" {
		return 1.0, nil
	}

	return 0.0, nil
}
