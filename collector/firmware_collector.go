package collector

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("firmware", newFirmwareCollector)
}

type firmwareCollector struct {
	description *prometheus.Desc
}

func newFirmwareCollector() routerOSCollector {
	labelNames := []string{"devicename", "name", "disabled", "version", "build_time"}
	c := &firmwareCollector{
		description: description("system", "package", "system packages version", labelNames),
	}

	return c
}

func (c *firmwareCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.description
}

func (c *firmwareCollector) collect(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/system/package/getall")
	if err != nil {
		return fmt.Errorf("fetch package error: %w", err)
	}

	for _, pkg := range reply.Re {
		ctx.ch <- prometheus.MustNewConstMetric(c.description, prometheus.GaugeValue, 1,
			ctx.device.Name, pkg.Map["name"], pkg.Map["disabled"], pkg.Map["version"],
			pkg.Map["build-time"])
	}

	return nil
}
