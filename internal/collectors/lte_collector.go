package collectors

import (
	"fmt"
	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/internal/metrics"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("lte", newLteCollector, "retrieves LTE interfaces metrics")
}

type lteCollector struct {
	metrics metrics.PropertyMetricList
}

func newLteCollector() RouterOSCollector {
	const prefix = "lte_interface"

	labelNames := []string{metrics.LabelInterface, "cell_id", "primary_band"}

	return &lteCollector{
		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyGaugeMetric(prefix, "rssi", labelNames...).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "rsrp", labelNames...).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "rsrq", labelNames...).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "sinr", labelNames...).Build(),
			metrics.NewPropertyGaugeMetric(prefix, "status", labelNames...).
				WithName("connected").
				WithConverter(metricFromLTEStatus).
				Build(),
		},
	}
}

func (c *lteCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *lteCollector) Collect(ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/interface/lte/print", "?disabled=false", "=.proplist=name")
	if err != nil {
		return fmt.Errorf("fetch lte interface names error: %w", err)
	}

	names := convert.ExtractPropertyFromReplay(reply, "name")

	var errs *multierror.Error

	for _, n := range names {
		if err := c.collectForInterface(n, ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}

func (c *lteCollector) collectForInterface(iface string, ctx *metrics.CollectorContext) error {
	reply, err := ctx.Client.Run("/interface/lte/monitor", "=number="+iface, "=once=",
		"=.proplist=current-cellid,primary-band,rssi,rsrp,rsrq,sinr,status")
	if err != nil {
		return fmt.Errorf("fetch %s lte interface statistics error: %w", iface, err)
	}

	if len(reply.Re) == 0 {
		return nil
	}

	re := reply.Re[0]

	primaryband := re.Map["primary-band"]
	if primaryband != "" {
		primaryband = strings.Fields(primaryband)[0]
	}

	lctx := ctx.WithLabels(iface, re.Map["current-cellid"], primaryband)

	if err := c.metrics.Collect(re, &lctx); err != nil {
		return fmt.Errorf("collect lte for %s error: %w", iface, err)
	}

	return nil
}

func metricFromLTEStatus(value string) (float64, error) {
	if value == "connected" {
		return 1.0, nil
	}

	return 0.0, nil
}
