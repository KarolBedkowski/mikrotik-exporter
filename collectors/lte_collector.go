package collectors

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

// TODO: need check

func init() {
	registerCollector("lte", newLteCollector, "retrieves LTE interfaces metrics")
}

type lteCollector struct {
	metrics PropertyMetricList
}

func newLteCollector() RouterOSCollector {
	const prefix = "lte_interface"

	labelNames := []string{"name", "address", "interface", "cell_id", "primary_band"}

	return &lteCollector{
		metrics: PropertyMetricList{
			NewPropertyGaugeMetric(prefix, "rssi", labelNames).Build(),
			NewPropertyGaugeMetric(prefix, "rsrp", labelNames).Build(),
			NewPropertyGaugeMetric(prefix, "rsrq", labelNames).Build(),
			NewPropertyGaugeMetric(prefix, "sinr", labelNames).Build(),
			NewPropertyGaugeMetric(prefix, "status", labelNames).
				WithName("connected").WithConverter(metricFromLTEStatus).Build(),
		},
	}
}

func (c *lteCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *lteCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/interface/lte/print", "?disabled=false", "=.proplist=name")
	if err != nil {
		return fmt.Errorf("fetch lte interface names error: %w", err)
	}

	names := extractPropertyFromReplay(reply, "name")

	var errs *multierror.Error

	for _, n := range names {
		if err := c.collectForInterface(n, ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}

func (c *lteCollector) collectForInterface(iface string, ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/interface/lte/monitor", "=number="+iface, "=once=",
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

	ctx = ctx.withLabels(iface, re.Map["current-cellid"], primaryband)

	if err := c.metrics.Collect(re, ctx); err != nil {
		return fmt.Errorf("collect ltr for %s error: %w", iface, err)
	}

	return nil
}

func metricFromLTEStatus(value string) (float64, error) {
	if value == "connected" {
		return 1.0, nil
	}

	return 0.0, nil
}
