package collectors

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mikrotik-exporter/config"
	"mikrotik-exporter/routeros"
	"mikrotik-exporter/routeros/proto"

	"github.com/go-kit/log/level"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	durationRegex *regexp.Regexp
	durationParts [6]time.Duration
)

var (
	ErrEmptyValue      = errors.New("empty value")
	ErrInvalidDuration = errors.New("invalid duration value sent to regex")
)

func init() {
	durationRegex = regexp.MustCompile(`(?:(\d*)w)?(?:(\d*)d)?(?:(\d*)h)?(?:(\d*)m)?(?:(\d*)s)?(?:(\d*)ms)?`)
	durationParts = [6]time.Duration{
		time.Hour * 168, time.Hour * 24, time.Hour, time.Minute, time.Second, time.Millisecond,
	}
}

type (
	ValueConverter     func(value string) (float64, error)
	TXRXValueConverter func(value string) (float64, float64, error)
)

// --------------------------------------------

func metricStringCleanup(in string) string {
	return strings.ReplaceAll(in, "-", "_")
}

func descriptionForPropertyName(prefix, property string, labelNames []string) *prometheus.Desc {
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

func cleanHostName(hostname string) string {
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

// --------------------------------------------

func splitStringToFloatsOnComma(metric string) (float64, float64, error) {
	return splitStringToFloats(metric, ",")
}

func splitStringToFloats(metric string, separator string) (float64, float64, error) {
	if metric == "" {
		return math.NaN(), math.NaN(), ErrEmptyValue
	}

	strs := strings.Split(metric, separator)
	switch len(strs) {
	case 0:
		return 0, 0, nil
	case 1:
		return math.NaN(), math.NaN(), fmt.Errorf("invalid input %v for split floats", metric) //nolint:goerr113
	}

	m1, err := strconv.ParseFloat(strs[0], 64)
	if err != nil {
		return math.NaN(), math.NaN(), fmt.Errorf("parse %v error: %w", metric, err)
	}

	m2, err := strconv.ParseFloat(strs[1], 64)
	if err != nil {
		return math.NaN(), math.NaN(), fmt.Errorf("parse %v error: %w", metric, err)
	}

	return m1, m2, nil
}

func metricFromDuration(duration string) (float64, error) {
	var totalDur time.Duration

	reMatch := durationRegex.FindAllStringSubmatch(duration, -1)

	// should get one and only one match back on the regex
	if len(reMatch) != 1 {
		return 0, fmt.Errorf("parse %s error: %w", duration, ErrInvalidDuration)
	}

	for idx, match := range reMatch[0][1:] {
		if match != "" {
			v, err := strconv.Atoi(match)
			if err != nil {
				return float64(0), fmt.Errorf("parse duration %s error: %w", duration, err)
			}

			totalDur += time.Duration(v) * durationParts[idx]
		}
	}

	return totalDur.Seconds(), nil
}

func metricFromString(value string) (float64, error) {
	return strconv.ParseFloat(value, 64) //nolint:wrapcheck
}

func metricFromBool(value string) (float64, error) {
	if value == "true" || value == "yes" {
		return 1.0, nil
	}

	return 0.0, nil
}

func metricConstantValue(value string) (float64, error) {
	_ = value

	return 1.0, nil
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
	property       string
	valueConverter ValueConverter
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
		_ = level.Debug(ctx.logger).Log("msg", fmt.Sprintf("property %s value not found", p.property),
			"property", p.property, "labels", ctx.labels)

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
	property       string
	valueConverter TXRXValueConverter
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
		_ = level.Debug(ctx.logger).Log("msg", fmt.Sprintf("property %s value not found", p.property),
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

// --------------------------------------------

type metricType int

const (
	metricCounter metricType = iota
	metricGauge
	metricRxTx
)

// PropertyMetricBuilder build metric collector that read given property from reply.
type PropertyMetricBuilder struct {
	prefix             string
	property           string
	valueConverter     ValueConverter
	rxTxValueConverter TXRXValueConverter
	metricType         metricType
	metricName         string
	metricHelp         string
	labels             []string
}

func NewPropertyCounterMetric(prefix, property string, labels []string) PropertyMetricBuilder {
	return PropertyMetricBuilder{
		prefix:     prefix,
		property:   property,
		metricType: metricCounter,
		labels:     labels,
	}
}

func NewPropertyGaugeMetric(prefix, property string, labels []string) PropertyMetricBuilder {
	return PropertyMetricBuilder{
		prefix:     prefix,
		property:   property,
		metricType: metricGauge,
		labels:     labels,
	}
}

func NewPropertyRxTxMetric(prefix, property string, labels []string) PropertyMetricBuilder {
	return PropertyMetricBuilder{
		prefix:     prefix,
		property:   property,
		metricType: metricRxTx,
		labels:     labels,
	}
}

func (p PropertyMetricBuilder) WithName(name string) PropertyMetricBuilder {
	p.metricName = name

	return p
}

func (p PropertyMetricBuilder) WithHelp(help string) PropertyMetricBuilder {
	p.metricHelp = help

	return p
}

func (p PropertyMetricBuilder) WithConverter(vc ValueConverter) PropertyMetricBuilder {
	if p.metricType == metricRxTx {
		panic("can't set ValueConverter for rxtx metric")
	}

	p.valueConverter = vc

	return p
}

func (p PropertyMetricBuilder) WithRxTxConverter(vc TXRXValueConverter) PropertyMetricBuilder {
	if p.metricType != metricRxTx {
		panic("can't set TXRXValueConverter for non-rxtx metric")
	}

	p.rxTxValueConverter = vc

	return p
}

func (p PropertyMetricBuilder) Build() PropertyMetric {
	metricName := p.metricName
	if metricName == "" {
		metricName = p.property
		if p.metricType == metricCounter || p.metricType == metricRxTx {
			metricName += "_total"
		}
	}

	metricHelp := p.metricHelp
	if metricHelp == "" {
		metricHelp = p.property + " for " + p.prefix
	}

	if p.valueConverter == nil {
		p.valueConverter = metricFromString
	}

	if p.rxTxValueConverter == nil {
		p.rxTxValueConverter = splitStringToFloatsOnComma
	}

	_ = level.Debug(config.GlobalLogger).Log("msg", "build metric",
		"name", metricName,
		"help", metricHelp,
		"prefix", p.prefix,
		"labels", fmt.Sprintf("%v", p.labels),
		"type", p.metricType,
		"property", p.property,
	)

	switch p.metricType {
	case metricCounter:
		desc := descriptionForPropertyNameHelpText(p.prefix, metricName, p.labels, metricHelp)

		return &simplePropertyMetric{desc, p.property, p.valueConverter, prometheus.GaugeValue}

	case metricGauge:
		desc := descriptionForPropertyNameHelpText(p.prefix, metricName, p.labels, metricHelp)

		return &simplePropertyMetric{desc, p.property, p.valueConverter, prometheus.GaugeValue}

	case metricRxTx:
		rxDesc := descriptionForPropertyNameHelpText(p.prefix, "rx_"+metricName, p.labels, metricHelp+" (RX)")
		txDesc := descriptionForPropertyNameHelpText(p.prefix, "tx_"+metricName, p.labels, metricHelp+" (TX)")

		return &rxTxPropertyMetric{rxDesc, txDesc, p.property, p.rxTxValueConverter}
	}

	panic("unknown metric type")
}

// --------------------------------------

// RetMetricBuilder build metric collector for `ret` returned value.
type RetMetricBuilder struct {
	prefix         string
	property       string
	valueConverter ValueConverter
	metricType     metricType
	metricName     string
	metricHelp     string
	labels         []string
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

	_ = level.Debug(config.GlobalLogger).Log("msg", "build metric",
		"name", metricName,
		"help", metricHelp,
		"prefix", r.prefix,
		"labels", fmt.Sprintf("%v", r.labels),
		"type", r.metricType,
		"property", r.property,
	)

	if r.metricType == metricGauge {
		desc := descriptionForPropertyNameHelpText(r.prefix, metricName, r.labels, metricHelp)

		return &retGaugeCollector{desc, r.property, valueConverter}
	}

	panic("unknown metric type")
}

// --------------------------------------

// RetMetric collect metrics from `ret` value retuned in reply.
type RetMetric interface {
	Describe(ch chan<- *prometheus.Desc)
	Collect(reply *routeros.Reply, ctx *CollectorContext) error
}

type retGaugeCollector struct {
	desc           *prometheus.Desc
	property       string
	valueConverter ValueConverter
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

func extractPropertyFromReplay(reply *routeros.Reply, name string) []string { //nolint:unparam
	values := make([]string, 0, len(reply.Re))

	for _, re := range reply.Re {
		values = append(values, re.Map[name])
	}

	return values
}
