package collectors

import (
	"fmt"

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
		description: description("system", "package_enabled", "system packages version and status",
			LabelDevName, LabelDevAddress, "name", "version", "build_time"),
	}
}

func (c *firmwareCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.description
}

func (c *firmwareCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/system/package/getall")
	if err != nil {
		return fmt.Errorf("fetch package error: %w", err)
	}

	for _, pkg := range reply.Re {
		enabled := 0.0
		if pkg.Map["disabled"] == "false" {
			enabled = 1.0
		}
		ctx.ch <- prometheus.MustNewConstMetric(c.description, prometheus.GaugeValue, enabled,
			ctx.device.Name, ctx.device.Address, pkg.Map["name"], pkg.Map["version"], pkg.Map["build-time"])
	}

	return nil
}
