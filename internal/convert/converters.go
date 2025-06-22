package convert

//
// converters.go
// Copyright (C) 2025 Karol Będkowski <Karol Będkowski@kkomp>
//
// Distributed under terms of the GPLv3 license.
//

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mikrotik-exporter/routeros"
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

// ----------------------------------------------------------------------------.
type (
	// ValueConverter convert value from api to metric.
	ValueConverter func(value string) (float64, error)
	// TXRXValueConverter convert value from api to metric; dedicated to tx/rx metrics.
	TXRXValueConverter func(value string) (float64, float64, error)
)

// ----------------------------------------------------------------------------

func ParseTS(value string) (float64, error) {
	if value == "" {
		return 0.0, nil
	}

	t, err := time.Parse("2006-01-02 15:04:05", value)
	if err == nil {
		return float64(t.Unix()), nil
	}

	t, err = time.Parse("Jan/02/2006 15:04:05", value)
	if err != nil {
		return 0.0, fmt.Errorf("parse time %s error: %w", value, err)
	}

	return float64(t.Unix()), nil
}

// ----------------------------------------------------------------------------

func SplitStringToFloatsOn(sep string) func(string) (float64, float64, error) {
	return func(metric string) (float64, float64, error) {
		return SplitStringToFloats(metric, sep)
	}
}

// splitStringToFloatsOnComma get two floats from `metric` separated by comma.
func SplitStringToFloatsOnComma(metric string) (float64, float64, error) {
	return SplitStringToFloats(metric, ",")
}

// splitStringToFloats split `metric` to two floats on `separator.
func SplitStringToFloats(metric, separator string) (float64, float64, error) {
	if metric == "" {
		return math.NaN(), math.NaN(), ErrEmptyValue
	}

	strs := strings.Split(metric, separator)
	switch len(strs) {
	case 0:
		return 0, 0, nil
	case 1:
		return math.NaN(), math.NaN(), InvalidInputError(fmt.Sprintf("can't split %v to floats", metric))
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

// ----------------------------------------------------------------------------

// metricFromDuration convert formatted `duration` to duration in seconds as float64.
func MetricFromDuration(duration string) (float64, error) {
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

// metricFromString convert string to float64.
func MetricFromString(value string) (float64, error) {
	return strconv.ParseFloat(value, 64) //nolint:wrapcheck
}

// metricFromBool return 1.0 if value is "true" or "yes"; 0.0 otherwise.
func MetricFromBool(value string) (float64, error) {
	if value == "true" || value == "yes" {
		return 1.0, nil
	}

	return 0.0, nil
}

// metricConstantValue always return 1.0.
func MetricConstantValue(value string) (float64, error) {
	_ = value

	return 1.0, nil
}

// ----------------------------------------------------------------------------

// ExtractPropertyFromReplay get all values from reply for property `name`.
func ExtractPropertyFromReplay(reply *routeros.Reply, name string) []string {
	values := make([]string, 0, len(reply.Re))

	for _, re := range reply.Re {
		values = append(values, re.Map[name])
	}

	return values
}

// ----------------------------------------------------------------------------

func TruncAfterAt(next ValueConverter) ValueConverter {
	return func(value string) (float64, error) {
		if value == "" {
			return 0.0, ErrEmptyValue
		}

		if i := strings.Index(value, "@"); i > -1 {
			value = value[:i]
		}

		return next(value)
	}
}
