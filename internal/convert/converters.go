package convert

//
// converters.go
// Copyright (C) 2025 Karol Będkowski <Karol Będkowski@kkomp>
//
// Distributed under terms of the GPLv3 license.
//

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"mikrotik-exporter/routeros"
)

// ----------------------------------------------------------------------------.
type (
	// ValueConverter convert value from api to metric.
	ValueConverter func(value string) (float64, error)
	// TXRXValueConverter convert value from api to metric; dedicated to tx/rx metrics.
	TXRXValueConverter func(value string) (float64, float64, error)
)

// ----------------------------------------------------------------------------

// ParseTS parse date from `value` (in UTC) into unix timestamp.
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

// ParseTS parse date from `value` (in UTC) into unix timestamp.
func ParseTSInLocation(value, timezone string) (float64, error) {
	if value == "" {
		return 0.0, nil
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return 0.0, fmt.Errorf("parse timezone %s error: %w", timezone, err)
	}

	t, err := time.ParseInLocation("2006-01-02 15:04:05", value, loc)
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

func indexFuncDigit(c rune) bool { return c >= '0' && c <= '9' }
func indexFuncChar(c rune) bool  { return c >= 'A' }

var ErrUnknownUnit = errors.New("unknown unit")

// adjustDuration create Duration by multiple duration by unit.
func adjustDuration(duration int, unit string) (time.Duration, error) {
	dur := time.Duration(duration)

	switch unit {
	case "w":
		dur *= time.Hour * 168 //nolint:mnd
	case "d":
		dur *= time.Hour * 24 //nolint:mnd
	case "h":
		dur *= time.Hour
	case "m":
		dur *= time.Minute
	case "s":
		dur *= time.Second
	case "ms":
		dur *= time.Millisecond
	case "us":
		dur *= time.Microsecond
	default:
		return 0, fmt.Errorf("parse dur unit %q error: %w", unit, ErrUnknownUnit)
	}

	return dur, nil
}

// getDurationParts read first parh of duration, parse it into Duration, return rest of input and
// optionally error.
// Example input: 1w3d3h42m53s13ms71us - return (Duration(1 week), "3d3h42m53s13ms71us", nil).
func getDurationParts(inp string) (time.Duration, string, error) {
	// find begging of unit
	unitIdx := strings.IndexFunc(inp, indexFuncChar)
	if unitIdx == -1 {
		return 0, "", fmt.Errorf("parse duration error: %w", ErrUnknownUnit)
	}

	// separate value and rest of input
	valuePart, rest := inp[:unitIdx], inp[unitIdx:]

	var unit string

	// find end of unit
	nextValueIdx := strings.IndexFunc(rest, indexFuncDigit)
	if nextValueIdx > -1 {
		unit, rest = rest[:nextValueIdx], rest[nextValueIdx:]
	} else {
		// no next value found
		rest, unit = "", rest
	}

	if unit == "" {
		return 0, "", fmt.Errorf("parse duration error: %w", ErrUnknownUnit)
	}

	if valuePart == "" {
		return 0, rest, nil
	}

	v, err := strconv.Atoi(valuePart)
	if err != nil {
		return 0, "", fmt.Errorf("parse duration %q error: %w", valuePart, err)
	}

	duration, err := adjustDuration(v, unit)

	return duration, rest, err
}

// metricFromDuration convert formatted `duration` to duration in seconds as float64.
func MetricFromDuration(duration string) (float64, error) {
	var totalDur time.Duration

	dur := duration

	for dur != "" {
		d, rest, err := getDurationParts(dur)
		if err != nil {
			return 0, fmt.Errorf("parse %s error: %w", duration, ErrInvalidDuration)
		}

		totalDur += d
		dur = rest
	}

	return totalDur.Seconds(), nil
}

// ----------------------------------------------------------------------------

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

// metricFromEnabled return 1.0 if value is "enabled""; 0.0 otherwise.
func MetricFromEnabled(value string) (float64, error) {
	if value == "enabled" {
		return 1.0, nil
	}

	return 0.0, nil
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

// ----------------------------------------------------------------------------

// UnixTimeFromDuration parse `duration` and return date now - `duration` as unix timestamp.
func UnixTimeFromDuration(duration string) (float64, error) {
	var totalDur time.Duration

	dur := duration

	for dur != "" {
		d, rest, err := getDurationParts(dur)
		if err != nil {
			return 0, fmt.Errorf("parse %s error: %w", duration, ErrInvalidDuration)
		}

		totalDur += d
		dur = rest
	}

	return float64(time.Now().Add(-totalDur).Unix()), nil
}
