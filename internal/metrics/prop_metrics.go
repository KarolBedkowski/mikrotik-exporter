package metrics

//
// prop_metrics.go
// Copyright (C) 2025 Karol Będkowski <Karol Będkowski@kkomp>
//
// Distributed under terms of the GPLv3 license.
//

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"mikrotik-exporter/internal/convert"
	"mikrotik-exporter/routeros/proto"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

// --------------------------------------

// simplePropertyMetric collect basic value for given property using `valueConverter` to convert
// it to float value. Should be created by PropertyMetricBuilder.
type simplePropertyMetric struct {
	desc           *prometheus.Desc
	valueConverter ValueConverter
	property       string
	valueType      prometheus.ValueType
}

func (p *simplePropertyMetric) Describe(ch chan<- *prometheus.Desc) {
	ch <- p.desc
}

func (p *simplePropertyMetric) Collect(reply *proto.Sentence, ctx *CollectorContext) error {
	if len(reply.Map) == 0 {
		ctx.Logger.Warn("empty replay from device",
			"property", p.property, "labels", ctx.Labels, "reply", reply)

		return nil
	}

	propertyVal, ok := reply.Map[p.property]
	if !ok {
		ctx.Logger.Debug(fmt.Sprintf("property %s value not found", p.property),
			"property", p.property, "labels", ctx.Labels, "reply", reply)

		return nil
	}

	if propertyVal == "" {
		return nil
	}

	if i := strings.Index(propertyVal, "@"); i > -1 {
		propertyVal = propertyVal[:i]
	}

	value, err := p.valueConverter(propertyVal)
	if err != nil {
		return fmt.Errorf("parse %v for property %s error: %w", propertyVal, p.property, err)
	}

	ctx.Ch <- prometheus.MustNewConstMetric(p.desc, p.valueType, value, ctx.Labels...)

	return nil
}

// --------------------------------------------

// rxTxPropertyMetric collect counter metrics from given property and put it into two metrics _tx i _rx.
type rxTxPropertyMetric struct {
	rxDesc         *prometheus.Desc
	txDesc         *prometheus.Desc
	valueConverter TXRXValueConverter
	property       string
}

func (p rxTxPropertyMetric) Describe(ch chan<- *prometheus.Desc) {
	ch <- p.rxDesc
	ch <- p.txDesc
}

func (p rxTxPropertyMetric) Collect(reply *proto.Sentence, ctx *CollectorContext) error {
	if len(reply.Map) == 0 {
		ctx.Logger.Warn("empty replay from device", "property", p.property, "labels", ctx.Labels, "reply", reply)

		return nil
	}

	propertyVal, ok := reply.Map[p.property]
	if !ok {
		ctx.Logger.Debug(fmt.Sprintf("property %s value not found", p.property),
			"property", p.property, "labels", ctx.Labels)

		return nil
	}

	if propertyVal == "" {
		return nil
	}

	tx, rx, err := p.valueConverter(propertyVal)
	if err != nil {
		return fmt.Errorf("Collect %v for property %s error: %w", propertyVal, p.property, err)
	}

	labels := ctx.Labels

	ctx.Ch <- prometheus.MustNewConstMetric(p.txDesc, prometheus.CounterValue, tx, labels...)
	ctx.Ch <- prometheus.MustNewConstMetric(p.rxDesc, prometheus.CounterValue, rx, labels...)

	return nil
}

// --------------------------------------------

type statusPropertyMetricDV struct {
	value string
	desc  *prometheus.Desc
}

// statusPropertyMetric collect gauge metrics from status.
type statusPropertyMetric struct {
	descs    []statusPropertyMetricDV
	property string
}

func newStatusPropertyMetric(prefix, metricName, property, metricHelp string, labels, values []string,
) *statusPropertyMetric {
	desc := make([]statusPropertyMetricDV, 0, len(values))
	for _, v := range values {
		desc = append(desc, statusPropertyMetricDV{
			v,
			descriptionForPropertyNameHelpText(prefix, metricName+"_"+v, labels, metricHelp),
		})
	}

	return &statusPropertyMetric{desc, property}
}

func (s statusPropertyMetric) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range s.descs {
		ch <- d.desc
	}
}

func (s statusPropertyMetric) Collect(reply *proto.Sentence, ctx *CollectorContext) error {
	if len(reply.Map) == 0 {
		ctx.Logger.Warn("empty replay from device", "property", s.property, "labels", ctx.Labels, "reply", reply)

		return nil
	}

	propertyVal, ok := reply.Map[s.property]
	if !ok {
		ctx.Logger.Debug(fmt.Sprintf("property %s value not found", s.property),
			"property", s.property, "labels", ctx.Labels)

		return nil
	}

	if propertyVal == "" {
		return nil
	}

	labels := ctx.Labels
	found := false

	for _, vd := range s.descs {
		val := 0.0
		if vd.value == propertyVal {
			val = 1
			found = true
		}

		ctx.Ch <- prometheus.MustNewConstMetric(vd.desc, prometheus.GaugeValue, val, labels...)
	}

	if !found {
		ctx.Logger.Debug(fmt.Sprintf("unknown property %s value", s.property),
			"property", s.property, "labels", ctx.Labels)
	}

	return nil
}

// --------------------------------------------

type constPropertyMetric struct {
	desc     *prometheus.Desc
	property string
}

func (p *constPropertyMetric) Describe(ch chan<- *prometheus.Desc) {
	ch <- p.desc
}

func (p *constPropertyMetric) Collect(reply *proto.Sentence, ctx *CollectorContext) error {
	if len(reply.Map) == 0 {
		ctx.Logger.Warn("empty replay from device", "property", p.property, "labels", ctx.Labels, "reply", reply)

		return nil
	}

	_, ok := reply.Map[p.property]
	if !ok {
		ctx.Logger.Debug(fmt.Sprintf("property %s value not found", p.property),
			"property", p.property, "labels", ctx.Labels, "reply_map", reply.Map)

		return nil
	}

	ctx.Ch <- prometheus.MustNewConstMetric(p.desc, prometheus.GaugeValue, 1.0, ctx.Labels...)

	return nil
}

// ----------------------------------------------------------------------------

type metricType int

const (
	metricCounter metricType = iota
	metricGauge
	metricRxTx
	metricStatus
	metricConst
	metricRet
)

// --------------------------------------------

// PropertyMetricBuilder build metric collector that read given property from reply.
type PropertyMetricBuilder struct {
	valueConverter     ValueConverter
	rxTxValueConverter TXRXValueConverter
	prefix             string
	property           string
	metricName         string
	metricHelp         string
	labels             []string
	metricType         metricType
	values             []string
}

// NewPropertyCounterMetric create new PropertyMetricBuilder for counter type metric with `prefix` and value from
// `property` with `labels`. First two labels are generated (device name and address) are added automatically
// but must be included in list; additional must filled.
func NewPropertyCounterMetric(prefix, property string, labels ...string) *PropertyMetricBuilder {
	return &PropertyMetricBuilder{
		prefix:     prefix,
		property:   property,
		metricType: metricCounter,
		labels:     append([]string{LabelDevName, LabelDevAddress}, labels...),
	}
}

// NewPropertyGaugeMetric create new PropertyMetricBuilder for gauge type metric with `prefix` and value from
// `property` with `labels`. First two labels are generated (device name and address) are added automatically
// but must be included in list; additional must filled.
func NewPropertyGaugeMetric(prefix, property string, labels ...string) *PropertyMetricBuilder {
	return &PropertyMetricBuilder{
		prefix:     prefix,
		property:   property,
		metricType: metricGauge,
		labels:     append([]string{LabelDevName, LabelDevAddress}, labels...),
	}
}

// NewPropertyRxTxMetric create new PropertyMetricBuilder for two counter type metrics (rx_, tx_) with `prefix`
// and values from `property`. First two labels are generated (device name and address)
// are added automatically but must be included in list; additional must filled.
func NewPropertyRxTxMetric(prefix, property string, labels ...string) *PropertyMetricBuilder {
	return &PropertyMetricBuilder{
		prefix:     prefix,
		property:   property,
		metricType: metricRxTx,
		labels:     append([]string{LabelDevName, LabelDevAddress}, labels...),
	}
}

// NewPropertyStatusMetric create new PropertyMetricBuilder that handle each value as separate metric with
// postfix `_<value>`. Value from property set 1 to matching metrics and 0 to rest.
func NewPropertyStatusMetric(prefix, property string, values []string, labels ...string) *PropertyMetricBuilder {
	return &PropertyMetricBuilder{
		prefix:     prefix,
		property:   property,
		metricType: metricStatus,
		labels:     append([]string{LabelDevName, LabelDevAddress}, labels...),
		values:     values,
	}
}

// NewPropertyConstMetric create new PropertyMetricBuilder that set metric to 1 always if `property` is in
// reply.
func NewPropertyConstMetric(prefix, property string, labels ...string) *PropertyMetricBuilder {
	return &PropertyMetricBuilder{
		prefix:     prefix,
		property:   property,
		metricType: metricConst,
		labels:     append([]string{LabelDevName, LabelDevAddress}, labels...),
	}
}

// NewPropertyRetMetric create new PropertyMetricBuilder for metrics received from `reply.Done` (i.e. `count-only`
// metrics stored in `reply.Done['ret']`.
func NewPropertyRetMetric(prefix, name string, labels ...string) *PropertyMetricBuilder {
	return &PropertyMetricBuilder{
		prefix:     prefix,
		property:   "ret",
		metricName: name,
		metricType: metricRet,
		labels:     append([]string{LabelDevName, LabelDevAddress}, labels...),
	}
}

// WithName set name for metric.
func (p *PropertyMetricBuilder) WithName(name string) *PropertyMetricBuilder {
	p.metricName = name

	return p
}

// WithHelp set help message for metric.
func (p *PropertyMetricBuilder) WithHelp(help string) *PropertyMetricBuilder {
	p.metricHelp = help

	return p
}

// WithConverter add converter that change value form property to float64.
func (p *PropertyMetricBuilder) WithConverter(vc ValueConverter) *PropertyMetricBuilder {
	if p.metricType != metricCounter && p.metricType != metricGauge && p.metricType != metricRet {
		panic("can't set ValueConverter for not counter/gauge metric")
	}

	p.valueConverter = vc

	return p
}

// WithRxTxConverter add converter for RxTx metric type.
func (p *PropertyMetricBuilder) WithRxTxConverter(vc TXRXValueConverter) *PropertyMetricBuilder {
	if p.metricType != metricRxTx {
		panic("can't set TXRXValueConverter for non-rxtx metric")
	}

	p.rxTxValueConverter = vc

	return p
}

// / Build create PropertyMetric from configuration.
func (p *PropertyMetricBuilder) Build() PropertyMetric {
	p.prepare()
	slog.Debug("build metric", "b", p)

	if err := p.check(); err != nil {
		slog.Error("build metrics error", "b", p, "err", err)
	}

	switch p.metricType {
	case metricCounter:
		desc := descriptionForPropertyNameHelpText(p.prefix, p.metricName, p.labels, p.metricHelp)

		return &simplePropertyMetric{desc, p.valueConverter, p.property, prometheus.GaugeValue}

	case metricGauge:
		desc := descriptionForPropertyNameHelpText(p.prefix, p.metricName, p.labels, p.metricHelp)

		return &simplePropertyMetric{desc, p.valueConverter, p.property, prometheus.GaugeValue}

	case metricRxTx:
		rxDesc := descriptionForPropertyNameHelpText(p.prefix, "rx_"+p.metricName, p.labels, p.metricHelp+" (RX)")
		txDesc := descriptionForPropertyNameHelpText(p.prefix, "tx_"+p.metricName, p.labels, p.metricHelp+" (TX)")

		return &rxTxPropertyMetric{rxDesc, txDesc, p.rxTxValueConverter, p.property}

	case metricStatus:
		return newStatusPropertyMetric(p.prefix, p.metricName, p.property, p.metricHelp, p.labels, p.values)

	case metricConst:
		desc := descriptionForPropertyNameHelpText(p.prefix, p.metricName, p.labels, p.metricHelp)

		return &constPropertyMetric{desc, p.property}

	case metricRet:
		desc := descriptionForPropertyNameHelpText(p.prefix, p.metricName, p.labels, p.metricHelp)

		return &simplePropertyMetric{desc, p.valueConverter, p.property, prometheus.GaugeValue}
	}

	panic("unknown metric type")
}

func (p *PropertyMetricBuilder) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("name", p.metricName),
		slog.String("help", p.metricHelp),
		slog.String("prefix", p.prefix),
		slog.Any("labels", p.labels),
		slog.Any("type", p.metricType),
		slog.String("property", p.property),
		slog.Any("values", p.values),
	)
}

func (p *PropertyMetricBuilder) prepare() {
	if p.metricName == "" {
		metricName := p.property
		if p.metricType == metricCounter || p.metricType == metricRxTx {
			metricName += "_total"
		}

		p.metricName = metricName
	}

	if p.metricHelp == "" {
		p.metricHelp = p.property + " for " + p.prefix
	}

	if p.valueConverter == nil {
		p.valueConverter = convert.MetricFromString
	}

	if p.rxTxValueConverter == nil {
		p.rxTxValueConverter = convert.SplitStringToFloatsOnComma
	}
}

func (p *PropertyMetricBuilder) check() error {
	var errs *multierror.Error

	// check for duplicated labels
	for i, l := range p.labels {
		if i == 0 {
			continue
		}

		for _, ll := range p.labels[:i-1] {
			if l == ll {
				errs = multierror.Append(errs, newBuilderError("duplicated label %q", l))
			}
		}
	}

	if !slices.Contains(p.labels, LabelDevName) {
		errs = multierror.Append(errs, newBuilderError("missing label %q", LabelDevName))
	}

	if !slices.Contains(p.labels, LabelDevAddress) {
		errs = multierror.Append(errs, newBuilderError("missing label %q", LabelDevAddress))
	}

	return errs.ErrorOrNil()
}
