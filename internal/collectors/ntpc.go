package collectors

import (
	"errors"
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
	enabled metrics.PropertyMetric
	status  metrics.PropertyMetric
	offset  metrics.PropertyMetric
}

func newNTPcCollector() RouterOSCollector {
	const prefix = "ntp_client"

	return &ntpcCollector{
		metrics.NewPropertyGaugeMetric(prefix, "enabled").WithConverter(convert.MetricFromBool).Build(),
		metrics.NewPropertyGaugeMetric(prefix, "status").
			WithName("synchronized").
			WithConverter(metricFromNtpStatus).
			Build(),
		metrics.NewPropertyGaugeMetric(prefix, "system-offset").WithConverter(msToSecConverter).Build(),
	}
}

func (c *ntpcCollector) Describe(ch chan<- *prometheus.Desc) {
	c.enabled.Describe(ch)
	c.status.Describe(ch)
	c.offset.Describe(ch)
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

	sentence := reply.Re[0]

	var errs error

	if err := c.enabled.Collect(sentence.Map, ctx); err != nil {
		errs = errors.Join(errs, fmt.Errorf("collect error: %w", err))
	}

	psStatus, _ := c.status.(metrics.PropertySimpleSet)
	if as := sentence.Map["active-server"]; as != "" {
		errs = errors.Join(errs, psStatus.Set(1.0, ctx))
	} else {
		errs = errors.Join(errs, psStatus.Set(0.0, ctx))
	}

	if la := sentence.Map["last-adjustment"]; la != "" {
		if dur, err := convert.MetricFromDuration(la); err != nil {
			errs = errors.Join(errs, fmt.Errorf("parse last-adjustment %q error: %w", la, err))
		} else {
			psOffset, _ := c.offset.(metrics.PropertySimpleSet)

			// RO7 return data as ms, so convert seconds to ms to match RO7 response.
			errs = errors.Join(errs, psOffset.Set(dur, ctx))
		}
	}

	return errs
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

	pl := metrics.PropertyMetricList{c.enabled, c.status, c.offset}
	if err := pl.Collect(reply.Re[0].Map, ctx); err != nil {
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
