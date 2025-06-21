package collectors

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"mikrotik-exporter/config"
	"mikrotik-exporter/routeros"
	"mikrotik-exporter/routeros/proto"
)

var (
	ErrEmptyValue      = errors.New("empty value")
	ErrInvalidDuration = errors.New("invalid duration value sent to regex")
)

type (
	// ValueConverter convert value from api to metric.
	ValueConverter func(value string) (float64, error)
	// TXRXValueConverter convert value from api to metric; dedicated to tx/rx metrics.
	TXRXValueConverter func(value string) (float64, float64, error)
)

// --------------------------------------------

type InvalidInputError string

func (i InvalidInputError) Error() string {
	return "invalid input: " + string(i)
}

// --------------------------------------------

func descriptionForPropertyName(prefix, property string, labelNames []string) *prometheus.Desc { //nolint:unused
	return descriptionForPropertyNameHelpText(prefix, property, labelNames, property)
}

func descriptionForPropertyNameHelpText(prefix, property string,
	labelNames []string, helpText string,
) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(config.Namespace, prefix, metricStringCleanup(property)),
		helpText,
		labelNames,
		nil,
	)
}

func description(prefix, name, helpText string, labelNames []string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(config.Namespace, prefix, metricStringCleanup(name)),
		helpText,
		labelNames,
		nil,
	)
}

// --------------------------------------

// PropertyMetric define metric collector that read values from configured property.
type PropertyMetric interface {
	Describe(ch chan<- *prometheus.Desc)
	Collect(reply *proto.Sentence, ctx *CollectorContext) error
}

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

func (p *simplePropertyMetric) Collect(reply *proto.Sentence,
	ctx *CollectorContext,
) error {
	propertyVal, ok := reply.Map[p.property]
	if !ok {
		ctx.logger.Debug(fmt.Sprintf("property %s value not found", p.property),
			"property", p.property, "labels", ctx.labels, "reply_map", reply.Map)

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

	ctx.ch <- prometheus.MustNewConstMetric(p.desc, p.valueType, value, ctx.labels...)

	return nil
}

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

func (p rxTxPropertyMetric) Collect(reply *proto.Sentence,
	ctx *CollectorContext,
) error {
	propertyVal, ok := reply.Map[p.property]
	if !ok {
		ctx.logger.Debug(fmt.Sprintf("property %s value not found", p.property),
			"property", p.property, "labels", ctx.labels)

		return nil
	}

	if propertyVal == "" {
		return nil
	}

	tx, rx, err := p.valueConverter(propertyVal)
	if err != nil {
		return fmt.Errorf("Collect %v for property %s error: %w", propertyVal, p.property, err)
	}

	labels := ctx.labels

	ctx.ch <- prometheus.MustNewConstMetric(p.txDesc, prometheus.CounterValue, tx, labels...)
	ctx.ch <- prometheus.MustNewConstMetric(p.rxDesc, prometheus.CounterValue, rx, labels...)

	return nil
}

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

func (s statusPropertyMetric) Collect(reply *proto.Sentence,
	ctx *CollectorContext,
) error {
	propertyVal, ok := reply.Map[s.property]
	if !ok {
		ctx.logger.Debug(fmt.Sprintf("property %s value not found", s.property),
			"property", s.property, "labels", ctx.labels)

		return nil
	}

	if propertyVal == "" {
		return nil
	}

	labels := ctx.labels
	found := false

	for _, vd := range s.descs {
		val := 0.0
		if vd.value == propertyVal {
			val = 1
			found = true
		}

		ctx.ch <- prometheus.MustNewConstMetric(vd.desc, prometheus.GaugeValue, val, labels...)
	}

	if !found {
		ctx.logger.Debug(fmt.Sprintf("unknown property %s value", s.property),
			"property", s.property, "labels", ctx.labels)
	}

	return nil
}

type constPropertyMetric struct {
	desc     *prometheus.Desc
	property string
}

func (p *constPropertyMetric) Describe(ch chan<- *prometheus.Desc) {
	ch <- p.desc
}

func (p *constPropertyMetric) Collect(reply *proto.Sentence,
	ctx *CollectorContext,
) error {
	_, ok := reply.Map[p.property]
	if !ok {
		ctx.logger.Debug(fmt.Sprintf("property %s value not found", p.property),
			"property", p.property, "labels", ctx.labels, "reply_map", reply.Map)

		return nil
	}

	ctx.ch <- prometheus.MustNewConstMetric(p.desc, prometheus.GaugeValue, 1.0, ctx.labels...)

	return nil
}

// --------------------------------------------

type metricType int

const (
	metricCounter metricType = iota
	metricGauge
	metricRxTx
	metricStatus
	metricConst
)

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
func NewPropertyCounterMetric(prefix, property string, labels []string) *PropertyMetricBuilder {
	return &PropertyMetricBuilder{
		prefix:     prefix,
		property:   property,
		metricType: metricCounter,
		labels:     labels,
	}
}

// NewPropertyGaugeMetric create new PropertyMetricBuilder for gauge type metric with `prefix` and value from
// `property` with `labels`. First two labels are generated (device name and address) are added automatically
// but must be included in list; additional must filled.
func NewPropertyGaugeMetric(prefix, property string, labels []string) *PropertyMetricBuilder {
	return &PropertyMetricBuilder{
		prefix:     prefix,
		property:   property,
		metricType: metricGauge,
		labels:     labels,
	}
}

// NewPropertyRxTxMetric create new PropertyMetricBuilder for two counter type metrics (rx_, tx_) with `prefix`
// and values from `property`. First two labels are generated (device name and address)
// are added automatically but must be included in list; additional must filled.
func NewPropertyRxTxMetric(prefix, property string, labels []string) *PropertyMetricBuilder {
	return &PropertyMetricBuilder{
		prefix:     prefix,
		property:   property,
		metricType: metricRxTx,
		labels:     labels,
	}
}

// NewPropertyStatusMetric create new PropertyMetricBuilder that handle each value as separate metric with
// postfix `_<value>`. Value from property set 1 to matching metrics and 0 to rest.
func NewPropertyStatusMetric(prefix, property string, labels []string, values ...string) *PropertyMetricBuilder {
	return &PropertyMetricBuilder{
		prefix:     prefix,
		property:   property,
		metricType: metricStatus,
		labels:     labels,
		values:     values,
	}
}

// NewPropertyConstMetric create new PropertyMetricBuilder that set metric to 1 always if `property` is in
// reply.
func NewPropertyConstMetric(prefix, property string, labels []string) *PropertyMetricBuilder {
	return &PropertyMetricBuilder{
		prefix:     prefix,
		property:   property,
		metricType: metricConst,
		labels:     labels,
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
	if p.metricType == metricRxTx {
		panic("can't set ValueConverter for rxtx metric")
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
	}

	panic("unknown metric type")
}

func (p *PropertyMetricBuilder) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("name", p.metricName),
		slog.String("help", p.metricHelp),
		slog.String("prefix", p.prefix),
		slog.String("labels", fmt.Sprintf("%v", p.labels)),
		slog.Int("type", int(p.metricType)),
		slog.String("property", p.property),
		slog.String("values", fmt.Sprintf("%v", p.values)),
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
		p.valueConverter = metricFromString
	}

	if p.rxTxValueConverter == nil {
		p.rxTxValueConverter = splitStringToFloatsOnComma
	}
}

// --------------------------------------

// RetMetricBuilder build metric collector for `ret` returned value.
type RetMetricBuilder struct {
	valueConverter ValueConverter
	prefix         string
	property       string
	metricName     string
	metricHelp     string
	labels         []string
	metricType     metricType
}

func NewRetGaugeMetric(prefix, property string, labels []string) RetMetricBuilder {
	return RetMetricBuilder{
		prefix:     prefix,
		property:   property,
		metricType: metricGauge,
		labels:     labels,
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

	valueConverter := metricFromString
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

func (r *retGaugeCollector) Collect(reply *routeros.Reply,
	ctx *CollectorContext,
) error {
	propertyVal := reply.Done.Map["ret"]
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

	ctx.ch <- prometheus.MustNewConstMetric(r.desc, prometheus.GaugeValue, value, ctx.labels...)

	return nil
}

// --------------------------------------

// PropertyMetricList is list of PropertyMetric that can be collected at once.
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

// extractPropertyFromReplay get all values from reply for property `name`.
func extractPropertyFromReplay(reply *routeros.Reply, name string) []string { //nolint:unparam
	values := make([]string, 0, len(reply.Re))

	for _, re := range reply.Re {
		values = append(values, re.Map[name])
	}

	return values
}

// --------------------------------------------

func countByProperty(re []*proto.Sentence, property string) map[string]int {
	counter := make(map[string]int)

	for _, re := range re {
		pool := re.Map[property]
		cnt := 1

		if count, ok := counter[pool]; ok {
			cnt = count + 1
		}

		counter[pool] = cnt
	}

	return counter
}
