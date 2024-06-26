package collectors

import (
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("cloud", newCloudCollector,
		"retrieves cloud services information")
}

type cloudCollector struct {
	ifaceStatusDesc *prometheus.Desc
}

func newCloudCollector() RouterOSCollector {
	labelNames := []string{"name", "address", "status"}

	return &cloudCollector{
		ifaceStatusDesc: descriptionForPropertyName("cloud", "status", labelNames),
	}
}

func (c *cloudCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.ifaceStatusDesc
}

func (c *cloudCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/ip/cloud/print")
	if err != nil {
		return fmt.Errorf("get cloud error: %w", err)
	}

	if len(reply.Re) != 1 {
		return nil
	}

	se := reply.Re[0]

	status, ok := se.Map["status"]
	if !ok {
		return nil
	}

	var statusUnknown, statusUpdated, statusUpdating, statusError float64

	status = strings.ToLower(status)

	switch {
	case status == "updated":
		statusUpdated = 1.0
	case strings.HasPrefix(status, "updating"):
		statusUpdating = 1.0
	case strings.HasPrefix(status, "error"):
		statusError = 1.0
	default:
		statusUnknown = 1.0
	}

	ctx.ch <- prometheus.MustNewConstMetric(c.ifaceStatusDesc, prometheus.GaugeValue,
		statusUnknown, ctx.device.Name, ctx.device.Address, "unknown")
	ctx.ch <- prometheus.MustNewConstMetric(c.ifaceStatusDesc, prometheus.GaugeValue,
		statusUpdated, ctx.device.Name, ctx.device.Address, "updated")
	ctx.ch <- prometheus.MustNewConstMetric(c.ifaceStatusDesc, prometheus.GaugeValue,
		statusUpdating, ctx.device.Name, ctx.device.Address, "updating")
	ctx.ch <- prometheus.MustNewConstMetric(c.ifaceStatusDesc, prometheus.GaugeValue,
		statusError, ctx.device.Name, ctx.device.Address, "error")

	return nil
}
