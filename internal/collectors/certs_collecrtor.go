package collectors

//
// Certs collector gather information about certificate expiration time.
//

import (
	"fmt"

	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/internal/metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("certs", newCertsCollector, "retrieves dns metrics")
}

type certsCollector struct {
	metrics metrics.PropertyMetricList
}

func newCertsCollector() RouterOSCollector {
	const prefix = "cert"

	labelNames := []string{"cert-name", "common-name", "issuer", "serial-number"}

	return &certsCollector{
		metrics: metrics.PropertyMetricList{
			metrics.NewPropertyGaugeMetric(prefix, "invalid-after", labelNames...).WithConverter(convert.ParseTS).Build(),
		},
	}
}

func (c *certsCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *certsCollector) Collect(ctx *metrics.CollectorContext) error {
	// NOTE: invalid-after is in local time.
	reply, err := ctx.Client.Run("/certificate/print", "=.proplist=name,common-name,issuer,serial-number,invalid-after")
	if err != nil {
		return fmt.Errorf("fetch certificate info error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.WithLabelsFromMap(re.Map, "name", "common-name", "issuer", "serial-number")

		if err := c.metrics.Collect(re.Map, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}
