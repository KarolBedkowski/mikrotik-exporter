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

func splitStringToFloats(metric string, sep ...string) (float64, float64, error) {
	if metric == "" {
		return math.NaN(), math.NaN(), ErrEmptyValue
	}

	separator := ","
	if len(sep) > 0 {
		separator = sep[0]
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

func parseDuration(duration string) (float64, error) {
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
				log.WithFields(log.Fields{
					"duration": duration,
					"value":    match,
					"error":    err,
				}).Error("error parsing duration field value")

				return float64(0), err
			}

			totalDur += time.Duration(v) * durationParts[idx]
		}
	}

	return totalDur.Seconds(), nil
}

func parseBool(value string) float64 {
	if value == "true" {
		return 1.0
	}

	return 0.0
}

type TXRXDecription struct {
	RXDesc *prometheus.Desc
	TXDesc *prometheus.Desc
}

func NewTXRXDescription(prefix, property string, labelNames []string) *TXRXDecription {
	return &TXRXDecription{
		RXDesc: descriptionForPropertyName(prefix, "rx_"+property, labelNames),
		TXDesc: descriptionForPropertyName(prefix, "tx_"+property, labelNames),
	}
}

func (t *TXRXDecription) describe(ch chan<- *prometheus.Desc) {
	ch <- t.RXDesc
	ch <- t.TXDesc
}

type ValueConverter func(value string) (float64, error)

func convertToFloat(value string) (float64, error) {
	return strconv.ParseFloat(value, 64)
}

func convertFromBool(value string) (float64, error) {
	if value == "true" {
		return 1.0, nil
	}

	return 0.0, nil
}

type retCollector struct {
	reply  *routeros.Reply
	labels []string
	ctx    *collectorContext
}

func newRetCollector(reply *routeros.Reply,
	ctx *collectorContext, labels ...string,
) *retCollector {
	labels = append([]string{ctx.device.Name, ctx.device.Address}, labels...)

	return &retCollector{
		reply:  reply,
		ctx:    ctx,
		labels: labels,
	}
}

func (r *retCollector) collectGaugeValue(
	desc *prometheus.Desc, converter ValueConverter,
) error {
	propertyVal := r.reply.Done.Map["ret"]
	if propertyVal == "" {
		return nil
	}

	if i := strings.Index(propertyVal, "@"); i > -1 {
		propertyVal = propertyVal[:i]
	}

	if converter == nil {
		converter = convertToFloat
	}

	value, err := converter(propertyVal)
	if err != nil {
		log.WithFields(log.Fields{
			"collector": r.ctx.collector,
			"device":    r.ctx.device.Name,
			"property":  "<RET>",
			"value":     propertyVal,
			"error":     err,
		}).Error("error parsing value")

		return fmt.Errorf("parse error: %w", err)
	}

	r.ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
		value, r.labels...)

	return nil
}

type propertyCollector struct {
	sentence *proto.Sentence
	labels   []string
	ctx      *collectorContext
}

func newPropertyCollector(sentence *proto.Sentence,
	ctx *collectorContext, labels ...string,
) *propertyCollector {
	labels = append([]string{ctx.device.Name, ctx.device.Address}, labels...)

	return &propertyCollector{
		sentence: sentence,
		ctx:      ctx,
		labels:   labels,
	}
}

func (p *propertyCollector) collectGaugeValue(
	desc *prometheus.Desc, property string,
	converter ValueConverter,
) error {
	propertyVal := p.sentence.Map[property]
	if propertyVal == "" {
		return nil
	}

	if i := strings.Index(propertyVal, "@"); i > -1 {
		propertyVal = propertyVal[:i]
	}

	if converter == nil {
		converter = convertToFloat
	}

	value, err := converter(propertyVal)
	if err != nil {
		log.WithFields(log.Fields{
			"collector": p.ctx.collector,
			"device":    p.ctx.device.Name,
			"property":  property,
			"value":     propertyVal,
			"error":     err,
		}).Error("error parsing value")

		return fmt.Errorf("parse error: %w", err)
	}

	p.ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
		value, p.labels...)

	return nil
}

type TXRXValueConverter func(value string, opts ...string) (float64, float64, error)

func (p *propertyCollector) collectRXTXCounterValue(
	desc *TXRXDecription, property string,
	conv TXRXValueConverter,
) error {
	propertyVal := p.sentence.Map[property]
	if propertyVal == "" {
		return nil
	}

	if conv == nil {
		conv = splitStringToFloats
	}

	tx, rx, err := conv(propertyVal)
	if err != nil {
		log.WithFields(log.Fields{
			"collector": p.ctx.collector,
			"device":    p.ctx.device.Name,
			"property":  property,
			"value":     propertyVal,
			"error":     err,
		}).Error("error parsing value")

		return fmt.Errorf("parse error: %w", err)
	}

	p.ctx.ch <- prometheus.MustNewConstMetric(desc.TXDesc, prometheus.CounterValue, tx, p.labels...)
	p.ctx.ch <- prometheus.MustNewConstMetric(desc.RXDesc, prometheus.CounterValue, rx, p.labels...)

	return nil
}

func (p *propertyCollector) collectCounterValue(
	desc *prometheus.Desc, property string,
	converter ValueConverter,
) error {
	propertyVal := p.sentence.Map[property]
	if propertyVal == "" {
		return nil
	}

	if i := strings.Index(propertyVal, "@"); i > -1 {
		propertyVal = propertyVal[:i]
	}

	if converter == nil {
		converter = convertToFloat
	}

	value, err := converter(propertyVal)
	if err != nil {
		log.WithFields(log.Fields{
			"collector": p.ctx.collector,
			"device":    p.ctx.device.Name,
			"property":  property,
			"value":     propertyVal,
			"error":     err,
		}).Error("error parsing value")

		return fmt.Errorf("parse error: %w", err)
	}

	p.ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue,
		value, p.labels...)

	return nil
}
