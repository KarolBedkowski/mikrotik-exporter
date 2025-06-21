package collectors

// Collect info about connections from /ip/service

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("service", newServiceConnCollector,
		"retrieves service connections metrics")
}

type serviceConnCollector struct {
	metrics *prometheus.Desc
}

func newServiceConnCollector() RouterOSCollector {
	const prefix = "service"

	return &serviceConnCollector{
		metrics: description(prefix, "active_connections_count", "number of active connection for service",
			LabelDevName, LabelDevAddress, "service"),
	}
}

func (c *serviceConnCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.metrics
}

func (c *serviceConnCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/ip/service/print", "?connection", "=.proplist=name")
	if err != nil {
		return fmt.Errorf("fetch service stats error: %w", err)
	}

	counter := make(map[string]int)

	// count rows per service
	for _, re := range reply.Re {
		service := re.Map["name"]
		cnt := 1

		if c, ok := counter[service]; ok {
			cnt = c + 1
		}

		counter[service] = cnt
	}

	for service, count := range counter {
		ctx.ch <- prometheus.MustNewConstMetric(c.metrics, prometheus.GaugeValue,
			float64(count), ctx.device.Name, ctx.device.Address, service)
	}

	return nil
}
