package convert

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitStringToFloats(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		input    string
		expected struct {
			f1 float64
			f2 float64
		}
		isNaN    bool
		hasError bool
	}{
		{
			"1.2,2.1",
			struct {
				f1 float64
				f2 float64
			}{1.2, 2.1},
			false,
			false,
		},
		{
			input:    "1.2,",
			isNaN:    true,
			hasError: true,
		},
		{
			input:    ",2.1",
			isNaN:    true,
			hasError: true,
		},
		{
			"1.2,2.1,3.2",
			struct {
				f1 float64
				f2 float64
			}{1.2, 2.1},
			false,
			false,
		},
		{
			input:    "",
			isNaN:    true,
			hasError: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("test_%+v", testCase), func(t *testing.T) {
			f1, f2, err := SplitStringToFloatsOnComma(testCase.input)

			switch testCase.hasError {
			case true:
				assert.Error(t, err)
			case false:
				assert.NoError(t, err)
			}

			switch testCase.isNaN {
			case true:
				assert.True(t, math.IsNaN(f1))
				assert.True(t, math.IsNaN(f2))
			case false:
				assert.Equal(t, testCase.expected.f1, f1)
				assert.Equal(t, testCase.expected.f2, f2)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		input    string
		output   float64
		hasError bool
	}{
		{"3d3h42m53s", 272573, false},
		{"15w3d3h42m53s", 9344573, false},
		{"42m53s", 2573, false},
		{"7w6d9h34m", 4786440, false},
		{"59", 0, true},
		{"s", 0, false},
		{"", 0, false},
		{"7ms523us", 0.007523, false},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("test_%+v", testCase), func(t *testing.T) {
			f, err := MetricFromDuration(testCase.input)

			switch testCase.hasError {
			case true:
				assert.Error(t, err, "tc: %v", testCase)
			case false:
				assert.NoError(t, err, "tc: %v", testCase)
			}

			assert.Equal(t, testCase.output, f, "tc: %v", testCase)
		})
	}
}

func TestGetDurationParts(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		input    string
		output   time.Duration
		rest     string
		hasError bool
	}{
		{"1w3d3h42m53s13ms71us", time.Duration(168) * time.Hour, "3d3h42m53s13ms71us", false},
		{"3d3h42m53s13ms71us", time.Duration(3*24) * time.Hour, "3h42m53s13ms71us", false},
		{"3h42m53s13ms71us", time.Duration(3) * time.Hour, "42m53s13ms71us", false},
		{"42m53s13ms71us", time.Duration(42) * time.Minute, "53s13ms71us", false},
		{"53s13ms71us", time.Duration(53) * time.Second, "13ms71us", false},
		{"13ms71us", time.Duration(13) * time.Millisecond, "71us", false},
		{"71us", time.Duration(71) * time.Microsecond, "", false},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("test_%+v", testCase), func(t *testing.T) {
			dur, rest, err := getDurationParts(testCase.input)

			switch testCase.hasError {
			case true:
				assert.Error(t, err)
			case false:
				assert.NoError(t, err)
			}

			assert.Equal(t, testCase.output, dur)
			assert.Equal(t, testCase.rest, rest)
		})
	}
}

func TestParseTs(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		input       string
		output      float64
		errorResult bool
	}{
		{"", 0.0, false},
		{"2025-06-02 11:32:45", 1748863965.0, false},
		{"Feb/03/2023 9:12:23", 1675415543.0, false},
		{"Feb/03/2023 9:12", 0.0, true},
		{"2023-14-01 11:29:12", 0.0, true},
		{"2023-1-01 25:29:12", 0.0, true},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("test_%+v", testCase), func(t *testing.T) {
			ts, err := ParseTS(testCase.input)

			if testCase.errorResult {
				require.Error(t, err)
			} else {
				require.Equal(t, testCase.output, ts)
			}
		})
	}
}

func TestParseTsInLocation(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		input       string
		output      float64
		errorResult bool
	}{
		{"", 0.0, false},
		{"2025-06-02 11:32:45", 1748856765.0, false},
		{"Feb/03/2023 9:12:23", 1675415543.0, false},
		{"Feb/03/2023 9:12", 0.0, true},
		{"2023-14-01 11:29:12", 0.0, true},
		{"2023-1-01 25:29:12", 0.0, true},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("test_%+v", testCase), func(t *testing.T) {
			ts, err := ParseTSInLocation(testCase.input, "Europe/Warsaw")

			if testCase.errorResult {
				require.Error(t, err)
			} else {
				require.Equal(t, testCase.output, ts)
			}
		})
	}
}
