package metrics

//
// ret.go
// Copyright (C) 2025 Karol Będkowski <Karol Będkowski@kkomp>
//
// Distributed under terms of the GPLv3 license.
//

import (
	"fmt"
	"log/slog"
	"strings"

	"mikrotik-exporter/internal/convert"

	"github.com/prometheus/client_golang/prometheus"
)

// --------------------------------------

// RetMetricBuilder build metric collector for `ret` returned value (like count).
type RetMetricBuilder struct {
	valueConverter ValueConverter
	prefix         string
	property       string
	metricName     string
	metricHelp     string
	labels         []string
	metricType     metricType
}

func NewRetGaugeMetric(prefix, property string, labels ...string) RetMetricBuilder {
	return RetMetricBuilder{
		prefix:     prefix,
		property:   property,
		metricType: metricGauge,
		labels:     append([]string{LabelDevName, LabelDevAddress}, labels...),
	}
}

func (r RetMetricBuilder) WithName(name string) RetMetricBuilder {
	r.metricName = name

	return r
}

func (r RetMetricBuilder) WithHelp(help string) RetMetricBuilder {
	r.metricHelp = help

	return r
}

func (r RetMetricBuilder) WithConverter(vc ValueConverter) RetMetricBuilder {
	r.valueConverter = vc

	return r
}

func (r RetMetricBuilder) Build() RetMetric {
	metricName := r.metricName
	if metricName == "" {
		metricName = r.property
		if r.metricType == metricCounter {
			metricName += "_total"
		}
	}

	metricHelp := r.metricHelp
	if metricHelp == "" {
		metricHelp = r.property + " for " + r.prefix
	}

	valueConverter := convert.MetricFromString
	if r.valueConverter != nil {
		valueConverter = r.valueConverter
	}

	slog.Debug("build metric",
		"name", metricName,
		"help", metricHelp,
		"prefix", r.prefix,
		"labels", fmt.Sprintf("%v", r.labels),
		"type", r.metricType,
		"property", r.property,
	)

	if r.metricType == metricGauge {
		desc := descriptionForPropertyNameHelpText(r.prefix, metricName, r.labels, metricHelp)

		return &retGaugeCollector{desc, valueConverter, r.property}
	}

	panic("unknown metric type")
}

// --------------------------------------

// RetMetric collect metrics from `ret` value returned in reply.
type RetMetric interface {
	Describe(ch chan<- *prometheus.Desc)
	Collect(reply *routeros.Reply, ctx *CollectorContext) error
}

type retGaugeCollector struct {
	desc           *prometheus.Desc
	valueConverter ValueConverter
	property       string
}

func (r *retGaugeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- r.desc
}

func (r *retGaugeCollector) Collect(reply map[string]string, ctx *CollectorContext) error {
	propertyVal := reply["ret"]
	if propertyVal == "" {
		return nil
	}

	if i := strings.Index(propertyVal, "@"); i > -1 {
		propertyVal = propertyVal[:i]
	}

	value, err := r.valueConverter(propertyVal)
	if err != nil {
		return fmt.Errorf("parse ret value %v error: %w", propertyVal, err)
	}

	ctx.Ch <- prometheus.MustNewConstMetric(r.desc, prometheus.GaugeValue, value, ctx.Labels...)

	return nil
}
