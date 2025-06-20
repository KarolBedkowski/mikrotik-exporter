package collectors

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

func parseTS(value string) (float64, error) {
	if value == "" {
		return 0.0, nil
	}

	t, err := time.Parse("2006-01-02 15:04:05", value)
	if err != nil {
		return 0.0, fmt.Errorf("parse time %s error: %w", value, err)
	}

	return float64(t.Unix()), nil
}

func metricStringCleanup(in string) string {
	return strings.ReplaceAll(in, "-", "_")
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

// splitStringToFloatsOnComma get two floats from `metric` separated by comma.
func splitStringToFloatsOnComma(metric string) (float64, float64, error) {
	return splitStringToFloats(metric, ",")
}

// splitStringToFloats split `metric` to two floats on `separator.
func splitStringToFloats(metric string, separator string) (float64, float64, error) {
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

// metricFromDuration convert formatted `duration` to duration in seconds as float64.
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

// metricFromString convert string to float64.
func metricFromString(value string) (float64, error) {
	return strconv.ParseFloat(value, 64) //nolint:wrapcheck
}

// metricFromBool return 1.0 if value is "true" or "yes"; 0.0 otherwise.
func metricFromBool(value string) (float64, error) {
	if value == "true" || value == "yes" {
		return 1.0, nil
	}

	return 0.0, nil
}

// metricConstantValue always return 1.0.
func metricConstantValue(value string) (float64, error) {
	_ = value

	return 1.0, nil
}
