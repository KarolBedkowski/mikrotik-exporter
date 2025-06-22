package metrics

import (
	"strconv"
	"strings"

	"mikrotik-exporter/internal/config"
	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/routeros/proto"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

type (
	// ValueConverter convert value from api to metric.
	ValueConverter = convert.ValueConverter
	// TXRXValueConverter convert value from api to metric; dedicated to tx/rx metrics.
	TXRXValueConverter = convert.TXRXValueConverter
)

// --------------------------------------------

func descriptionForPropertyNameHelpText(prefix, property string,
	labelNames []string, helpText string,
) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(config.Namespace, prefix, MetricStringCleanup(property)),
		helpText,
		labelNames,
		nil,
	)
}

func Description(prefix, name, helpText string, labelNames ...string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(config.Namespace, prefix, MetricStringCleanup(name)),
		helpText,
		labelNames,
		nil,
	)
}

// --------------------------------------

// PropertyMetric define metric collector that read values from configured property.
type PropertyMetric interface {
	Describe(ch chan<- *prometheus.Desc)
	Collect(re *proto.Sentence, ctx *CollectorContext) error
}

// --------------------------------------------

// metrics.PropertyMetricList is list of PropertyMetric that can be collected at once.
type PropertyMetricList []PropertyMetric

func (p PropertyMetricList) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range p {
		m.Describe(ch)
	}
}

func (p PropertyMetricList) Collect(re *proto.Sentence, ctx *CollectorContext) error {
	var errs *multierror.Error

	for _, m := range p {
		if err := m.Collect(re, ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}

// --------------------------------------------

func MetricStringCleanup(in string) string {
	return strings.ReplaceAll(in, "-", "_")
}

func CleanHostName(hostname string) string {
	if hostname != "" {
		if hostname[0] == '"' {
			hostname = hostname[1 : len(hostname)-1]
		}

		// QuoteToASCII because of broken DHCP clients
		hostname = strconv.QuoteToASCII(hostname)
		hostname = hostname[1 : len(hostname)-1]
	}

	return hostname
}
