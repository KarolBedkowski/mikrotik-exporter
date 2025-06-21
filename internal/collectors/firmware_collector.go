package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/metrics"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("firmware", newFirmwareCollector, "retrieves firmware version")
}

type firmwareCollector struct {
	description *prometheus.Desc
}

func newFirmwareCollector() RouterOSCollector {
	return &firmwareCollector{
		description: metrics.Description("system", "package_enabled", "system packages version and status",
			metrics.LabelDevName, metrics.LabelDevAddress, "name", "version", "build_time"),
	}
}

func (c *firmwareCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.description
}

func (c *firmwareCollector) Collect(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/system/package/getall")
	if err != nil {
		return fmt.Errorf("fetch package error: %w", err)
	}

	for _, pkg := range reply.Re {
		enabled := 0.0
		if pkg.Map["disabled"] == "false" {
			enabled = 1.0
		}
		ctx.Ch <- prometheus.MustNewConstMetric(c.description, prometheus.GaugeValue, enabled,
			ctx.Device.Name, ctx.Device.Address, pkg.Map["name"], pkg.Map["version"], pkg.Map["build-time"])
	}

	return nil
}
