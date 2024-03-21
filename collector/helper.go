package collector

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/KarolBedkowski/routeros-go-client"
	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var (
	durationRegex *regexp.Regexp
	durationParts [6]time.Duration
)

func init() {
	durationRegex = regexp.MustCompile(`(?:(\d*)w)?(?:(\d*)d)?(?:(\d*)h)?(?:(\d*)m)?(?:(\d*)s)?(?:(\d*)ms)?`)
	durationParts = [6]time.Duration{
		time.Hour * 168, time.Hour * 24, time.Hour, time.Minute, time.Second, time.Millisecond,
	}
}

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
		prometheus.BuildFQName(namespace, prefix, metricStringCleanup(property)),
		helpText,
		labelNames,
		nil,
	)
}

func description(prefix, name, helpText string, labelNames []string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(namespace, prefix, metricStringCleanup(name)),
		helpText,
		labelNames,
		nil,
	)
}

var ErrEmptyValue = errors.New("empty value")

func splitStringToFloatsOnComma(metric string) (float64, float64, error) {
	return splitStringToFloats(metric, ",")
}

func splitStringToFloats(metric string, separator string) (float64, float64, error) {
	if metric == "" {
		return math.NaN(), math.NaN(), ErrEmptyValue
	}

	strs := strings.Split(metric, separator)
	if len(strs) == 0 {
		return 0, 0, nil
	}

	m1, err := strconv.ParseFloat(strs[0], 64)
	if err != nil {
		return math.NaN(), math.NaN(), err
	}

	m2, err := strconv.ParseFloat(strs[1], 64)
	if err != nil {
		return math.NaN(), math.NaN(), err
	}

	return m1, m2, nil
}

var ErrInvalidDuration = errors.New("invalid duration value sent to regex")

func metricFromDuration(duration string) (float64, error) {
	var totalDur time.Duration

	reMatch := durationRegex.FindAllStringSubmatch(duration, -1)

	// should get one and only one match back on the regex
	if len(reMatch) != 1 {
		return 0, ErrInvalidDuration
	}

	for idx, match := range reMatch[0][1:] {
		if match != "" {
			v, err := strconv.Atoi(match)
			if err != nil {
				return float64(0), err
			}

			totalDur += time.Duration(v) * durationParts[idx]
		}
	}

	return totalDur.Seconds(), nil
}

type ValueConverter func(value string) (float64, error)

func metricFromString(value string) (float64, error) {
	return strconv.ParseFloat(value, 64)
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

type propertyMetricCollector interface {
	describe(ch chan<- *prometheus.Desc)
	collect(reply *proto.Sentence, ctx *collectorContext) error
}

type propertyCollector struct {
	desc           *prometheus.Desc
	property       string
	valueConverter ValueConverter
	valueType      prometheus.ValueType
}

func (p *propertyCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- p.desc
}

func (p *propertyCollector) collect(reply *proto.Sentence,
	ctx *collectorContext,
) error {
	propertyVal, ok := reply.Map[p.property]
	if !ok {
		log.WithFields(log.Fields{
			"collector": ctx.collector,
			"device":    ctx.device.Name,
			"property":  p.property,
			"labels":    ctx.labels,
		}).Debugf("property %s value not found", p.property)

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

type propertyRxTxCollector struct {
	rxDesc         *prometheus.Desc
	txDesc         *prometheus.Desc
	property       string
	valueConverter TXRXValueConverter
}

func (p *propertyRxTxCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- p.rxDesc
	ch <- p.txDesc
}

func (p *propertyRxTxCollector) collect(reply *proto.Sentence,
	ctx *collectorContext,
) error {
	propertyVal, ok := reply.Map[p.property]
	if !ok {
		log.WithFields(log.Fields{
			"collector": ctx.collector,
			"device":    ctx.device.Name,
			"property":  p.property,
			"labels":    ctx.labels,
		}).Debugf("property %s not found value", p.property)

		return nil
	}

	if propertyVal == "" {
		return nil
	}

	tx, rx, err := p.valueConverter(propertyVal)
	if err != nil {
		return fmt.Errorf("collect %v for property %s error: %w", propertyVal, p.property, err)
	}

	labels := ctx.labels

	ctx.ch <- prometheus.MustNewConstMetric(p.txDesc, prometheus.CounterValue, tx, labels...)
	ctx.ch <- prometheus.MustNewConstMetric(p.rxDesc, prometheus.CounterValue, rx, labels...)

	return nil
}

type metricType int

const (
	metricCounter metricType = iota
	metricGauge
	metricRxTx
)

type TXRXValueConverter func(value string) (float64, float64, error)

type propertyMetricBuilder struct {
	prefix             string
	property           string
	valueConverter     ValueConverter
	rxTxValueConverter TXRXValueConverter
	metricType         metricType
	metricName         string
	metricHelp         string
	labels             []string
}

func newPropertyCounterMetric(prefix, property string, labels []string) *propertyMetricBuilder {
	return &propertyMetricBuilder{
		prefix:     prefix,
		property:   property,
		metricType: metricCounter,
		labels:     labels,
	}
}

func newPropertyGaugeMetric(prefix, property string, labels []string) *propertyMetricBuilder {
	return &propertyMetricBuilder{
		prefix:     prefix,
		property:   property,
		metricType: metricGauge,
		labels:     labels,
	}
}

func newPropertyRxTxMetric(prefix, property string, labels []string) *propertyMetricBuilder {
	return &propertyMetricBuilder{
		prefix:     prefix,
		property:   property,
		metricType: metricRxTx,
		labels:     labels,
	}
}

func (p *propertyMetricBuilder) withName(name string) *propertyMetricBuilder {
	p.metricName = name

	return p
}

func (p *propertyMetricBuilder) withHelp(help string) *propertyMetricBuilder {
	p.metricHelp = help

	return p
}

func (p *propertyMetricBuilder) withConverter(vc ValueConverter) *propertyMetricBuilder {
	if p.metricType == metricRxTx {
		panic("can't set ValueConverter for rxtx metric")
	}

	p.valueConverter = vc

	return p
}

func (p *propertyMetricBuilder) withRxTxConverter(vc TXRXValueConverter) *propertyMetricBuilder {
	if p.metricType != metricRxTx {
		panic("can't set TXRXValueConverter for non-rxtx metric")
	}

	p.rxTxValueConverter = vc

	return p
}

func (p *propertyMetricBuilder) build() propertyMetricCollector {
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

	log.WithFields(log.Fields{
		"name":     metricName,
		"help":     metricHelp,
		"prefix":   p.prefix,
		"labels":   p.labels,
		"type":     p.metricType,
		"property": p.property,
	}).Debug("build metric")

	switch p.metricType {
	case metricCounter:
		desc := descriptionForPropertyNameHelpText(p.prefix, metricName, p.labels, metricHelp)

		return &propertyCollector{desc, p.property, p.valueConverter, prometheus.GaugeValue}

	case metricGauge:
		desc := descriptionForPropertyNameHelpText(p.prefix, metricName, p.labels, metricHelp)

		return &propertyCollector{desc, p.property, p.valueConverter, prometheus.GaugeValue}

	case metricRxTx:
		rxDesc := descriptionForPropertyNameHelpText(p.prefix, "rx_"+metricName, p.labels, metricHelp+" (RX)")
		txDesc := descriptionForPropertyNameHelpText(p.prefix, "tx_"+metricName, p.labels, metricHelp+" (TX)")

		return &propertyRxTxCollector{rxDesc, txDesc, p.property, p.rxTxValueConverter}
	}

	panic("unknown metric type")
}

type retMetricBuilder struct {
	prefix         string
	property       string
	valueConverter ValueConverter
	metricType     metricType
	metricName     string
	metricHelp     string
	labels         []string
}

func newRetGaugeMetric(prefix, property string, labels []string) *retMetricBuilder {
	return &retMetricBuilder{
		prefix:     prefix,
		property:   property,
		metricType: metricGauge,
		labels:     labels,
	}
}

func (r *retMetricBuilder) withName(name string) *retMetricBuilder { //nolint:unused
	r.metricName = name

	return r
}

func (r *retMetricBuilder) withHelp(help string) *retMetricBuilder {
	r.metricHelp = help

	return r
}

func (r *retMetricBuilder) withConverter(vc ValueConverter) *retMetricBuilder { //nolint:unused
	r.valueConverter = vc

	return r
}

func (r *retMetricBuilder) build() retMetricCollector {
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

	log.WithFields(log.Fields{
		"name":     metricName,
		"help":     metricHelp,
		"prefix":   r.prefix,
		"labels":   r.labels,
		"type":     r.metricType,
		"property": r.property,
	}).Debug("build metric")

	if r.metricType == metricGauge {
		desc := descriptionForPropertyNameHelpText(r.prefix, metricName, r.labels, metricHelp)

		return &retGaugeCollector{desc, r.property, valueConverter}
	}

	panic("unknown metric type")
}

type retMetricCollector interface {
	describe(ch chan<- *prometheus.Desc)
	collect(reply *routeros.Reply, ctx *collectorContext) error
}

type retGaugeCollector struct {
	desc           *prometheus.Desc
	property       string
	valueConverter ValueConverter
}

func (r *retGaugeCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- r.desc
}

func (r *retGaugeCollector) collect(reply *routeros.Reply,
	ctx *collectorContext,
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

type propertyMetricList []propertyMetricCollector

func (p propertyMetricList) describe(ch chan<- *prometheus.Desc) {
	for _, m := range p {
		m.describe(ch)
	}
}

func (p propertyMetricList) collect(re *proto.Sentence, ctx *collectorContext) error {
	var errs *multierror.Error

	for _, m := range p {
		if err := m.collect(re, ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}

func extractPropertyFromReplay(reply *routeros.Reply, name string) []string { //nolint:unparam
	values := make([]string, 0, len(reply.Re))

	for _, re := range reply.Re {
		values = append(values, re.Map[name])
	}

	return values
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
