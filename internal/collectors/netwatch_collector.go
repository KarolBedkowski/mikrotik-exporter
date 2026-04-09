package collectors

import (
	"errors"
	"fmt"

	"mikrotik-exporter/internal/metrics"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("netwatch", newNetwatchCollector, "retrieves Netwatch status")
}

type netwatchCollector struct {
	metric metrics.PropertyMetric
}

func newNetwatchCollector() RouterOSCollector {
	labelNames := []string{"host", metrics.LabelComment, "status"}

	return &netwatchCollector{
		metric: metrics.NewPropertyConstMetric("netwatch", "status", labelNames...).Build(),
	}
}

func (c *netwatchCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metric.Describe(ch)
}

func (c *netwatchCollector) Collect(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/tool/netwatch/print",
		"?disabled=false",
		"=.proplist=host,comment,status")
	if err != nil {
		return fmt.Errorf("fetch netwatch error: %w", err)
	}

	var errs error

	for _, re := range reply.Re {
		lctx := ctx.WithLabelsFromMap(re.Map, "host", "comment", "status")
		if err := c.metric.Collect(re.Map, &lctx); err != nil {
			errs = errors.Join(errs, fmt.Errorf("collect error %w", err))
		}
	}

	return errs
}
