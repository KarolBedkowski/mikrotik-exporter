package collectors

import (
	"fmt"

	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("ip", newIPCollector, "retrieves ip metrics")
}

type ipCollector struct {
	metrics metrics.PropertyMetricList
}

func newIPCollector() RouterOSCollector {
	const prefix = "ip"

	return &ipCollector{
		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyCounterMetric(prefix, "ipv4-fast-path-active").
				WithConverter(convert.MetricFromBool).
				Build(),
			metrics.NewPropertyCounterMetric(prefix, "ipv4-fast-path-bytes").Build(),
			metrics.NewPropertyCounterMetric(prefix, "ipv4-fast-path-packets").Build(),
			metrics.NewPropertyCounterMetric(prefix, "ipv4-fasttrack-active").
				WithConverter(convert.MetricFromBool).
				Build(),
			metrics.NewPropertyCounterMetric(prefix, "ipv4-fasttrack-bytes").Build(),
			metrics.NewPropertyCounterMetric(prefix, "ipv4-fasttrack-packets").Build(),
		},
	}
}

func (c *ipCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *ipCollector) Collect(ctx *metrics.CollectorContext) error {
	// list of props must contain all values for labels and metrics
	reply, err := ctx.Client.Run("/ip/settings/print",
		"=.proplist=ipv4-fast-path-active,ipv4-fast-path-bytes,ipv4-fast-path-packets,"+
			"ipv4-fasttrack-active,ipv4-fasttrack-bytes,ipv4-fasttrack-packets")
	if err != nil {
		return fmt.Errorf("fetch ip settings error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		// collect metrics using context
		if err := c.metrics.Collect(re.Map, ctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}
