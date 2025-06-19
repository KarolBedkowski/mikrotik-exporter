package collectors

//
// Certs collector gather information about certificate expiration time.
//

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("certs", newCertsCollector,
		"retrieves dns metrics")
}

type certsCollector struct {
	metrics PropertyMetricList
}

func newCertsCollector() RouterOSCollector {
	const prefix = "cert"

	labelNames := []string{"name", "address", "cert-name", "common-name", "issuer", "serial-number"}

	collector := &certsCollector{
		metrics: PropertyMetricList{
			NewPropertyGaugeMetric(prefix, "invalid-after", labelNames).WithName("invalid_after").
				WithConverter(parseTS).Build(),
		},
	}

	return collector
}

func (c *certsCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *certsCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/certificate/print", "=.proplist=name,common-name,issuer,serial-number,invalid-after")
	if err != nil {
		return fmt.Errorf("fetch certificate info error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.withLabelsFromMap(re.Map, "name", "common-name", "issuer", "serial-number")

		if err := c.metrics.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}
