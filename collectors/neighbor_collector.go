package collectors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("neighbor", newNeighborCollector,
		"retrieves neighbor metrics")
}

type neighborCollector struct {
	metrics PropertyMetricList
}

func newNeighborCollector() RouterOSCollector {
	const prefix = "neighbor"

	// values for first two labels (device name and address) are added automatically;
	// rest must be loaded in Collect.
	labelNames := []string{
		"name", "address",
		"about", "address4", "discovered-by", "interface", "ipv6", "platform", "software-id", "version", "neighbor-address",
		"address6", "board", "identity", "interface-name", "mac-address", "system-caps", "system-description",
	}

	return &neighborCollector{
		metrics: PropertyMetricList{
			NewPropertyConstMetric(prefix, "address", labelNames).WithName("entry").Build(),
		},
	}
}

func (c *neighborCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c *neighborCollector) Collect(ctx *CollectorContext) error {
	reply, err := ctx.client.Run("/ip/neighbor/print",
		"=.proplist=about,address4,discovered-by,interface,ipv6,platform,software-id,version,address,"+
			"address6,board,identity,interface-name,mac-address,system-caps,system-description")
	if err != nil {
		return fmt.Errorf("fetch neighbor error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		lctx := ctx.withLabelsFromMap(re.Map,
			"about", "address4", "discovered-by", "interface", "ipv6", "platform", "software-id", "version",
			"address", "address6", "board", "identity", "interface-name", "mac-address", "system-caps",
			"system-description",
		)

		if err := c.metrics.Collect(re, &lctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error: %w", err))
		}
	}

	return errs.ErrorOrNil()
}
