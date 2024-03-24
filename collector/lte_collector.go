package collector

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
	metrics propertyMetricList
}

func newLteCollector() routerOSCollector {
	const prefix = "lte_interface"

	labelNames := []string{"name", "address", "interface", "cell_id", "primary_band"}

	return &lteCollector{
		metrics: propertyMetricList{
			newPropertyGaugeMetric(prefix, "rssi", labelNames).build(),
			newPropertyGaugeMetric(prefix, "rsrp", labelNames).build(),
			newPropertyGaugeMetric(prefix, "rsrq", labelNames).build(),
			newPropertyGaugeMetric(prefix, "sinr", labelNames).build(),
			newPropertyGaugeMetric(prefix, "status", labelNames).
				withName("connected").withConverter(metricFromLTEStatus).build(),
		},
	}
}

func (c *lteCollector) describe(ch chan<- *prometheus.Desc) {
	c.metrics.describe(ch)
}

func (c *lteCollector) collect(ctx *collectorContext) error {
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

func (c *lteCollector) collectForInterface(iface string, ctx *collectorContext) error {
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

	if err := c.metrics.collect(re, ctx); err != nil {
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
