package metrics

import (
	"fmt"
	"log/slog"
	"testing"

	"mikrotik-exporter/internal/config"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func TestSimplePropertyGauge(t *testing.T) {
	b := NewPropertyGaugeMetric("test", "property1", "lab1", "lab2")
	sp := b.Build()

	chdesc := make(chan *prometheus.Desc, 1)
	defer close(chdesc)

	sp.Describe(chdesc)

	desc := <-chdesc
	assert.Equal(t, desc.String(),
		"Desc{fqName: \"mikrotik_test_property1\", help: \"property1 for test\", "+
			"constLabels: {}, variableLabels: {dev_name,dev_address,lab1,lab2}}",
	)

	chout := make(chan prometheus.Metric, 1)
	defer close(chout)

	device := config.Device{Name: "devname", Address: "devaddress"}
	ctx := NewCollectorContext(chout, &device, nil, "coltest", slog.Default(), nil)
	sent := map[string]string{"property1": "123.23", "aa": "valaa", "bb": ""}
	lctx := ctx.WithLabelsFromMap(sent, "aa", "bb")

	err := sp.Collect(sent, &lctx)
	assert.NoError(t, err)

	metric, labels, err := collectMetric(chout)
	assert.NoError(t, err)

	assert.Equal(t, 123.23, metric.Gauge.GetValue())
	assert.Equal(t, 4, len(labels))
	assert.Equal(t, "devname", labels["dev_name"])
	assert.Equal(t, "devaddress", labels["dev_address"])
	assert.Equal(t, "valaa", labels["lab1"])
	assert.Equal(t, "", labels["lab2"])
}

func TestSimplePropertyCounter(t *testing.T) {
	b := NewPropertyCounterMetric("test", "property1").WithName("metric1")
	sp := b.Build()

	chdesc := make(chan *prometheus.Desc, 1)
	defer close(chdesc)

	sp.Describe(chdesc)

	desc := <-chdesc
	assert.Equal(t, desc.String(),
		"Desc{fqName: \"mikrotik_test_metric1\", help: \"property1 for test\", "+
			"constLabels: {}, variableLabels: {dev_name,dev_address}}",
	)

	chout := make(chan prometheus.Metric, 1)
	defer close(chout)

	device := config.Device{Name: "devname2", Address: "devaddress2"}
	ctx := NewCollectorContext(chout, &device, nil, "coltest", slog.Default(), nil)
	sent := map[string]string{"property1": "123.567", "aa": "valaa", "bb": ""}

	err := sp.Collect(sent, &ctx)
	assert.NoError(t, err)

	metric, labels, err := collectMetric(chout)
	assert.NoError(t, err)

	assert.Equal(t, 123.567, metric.Counter.GetValue(), "%+v", metric)
	assert.Equal(t, 2, len(labels))
	assert.Equal(t, "devname2", labels["dev_name"])
	assert.Equal(t, "devaddress2", labels["dev_address"])
}

func TestSimplePropertyConsts(t *testing.T) {
	b := NewPropertyConstMetric("test", "property1").WithName("metric1")
	sp := b.Build()

	chdesc := make(chan *prometheus.Desc, 1)
	defer close(chdesc)

	sp.Describe(chdesc)

	desc := <-chdesc
	assert.Equal(t, desc.String(),
		"Desc{fqName: \"mikrotik_test_metric1\", help: \"property1 for test\", "+
			"constLabels: {}, variableLabels: {dev_name,dev_address}}",
	)

	chout := make(chan prometheus.Metric, 1)
	defer close(chout)

	device := config.Device{Name: "devname2", Address: "devaddress2"}
	ctx := NewCollectorContext(chout, &device, nil, "coltest", slog.Default(), nil)

	testCase := []struct {
		input map[string]string
		value float64
	}{
		{map[string]string{"property1": "123.567", "aa": "valaa", "bb": ""}, 1.0},
		{map[string]string{"property1": "", "aa": "valaa", "bb": ""}, 1.0},
		{map[string]string{"aa": "valaa", "bb": ""}, 0.0},
	}

	for _, tc := range testCase {
		err := sp.Collect(tc.input, &ctx)
		assert.NoError(t, err)

		if tc.value == 0.0 {
			assert.Equal(t, 0, len(chout))
			continue
		}

		metric, labels, err := collectMetric(chout)
		assert.NoError(t, err)

		assert.Equal(t, tc.value, metric.Gauge.GetValue(), "%+v -> %+v", tc, metric)
		assert.Equal(t, len(labels), 2)
		assert.Equal(t, "devname2", labels["dev_name"])
		assert.Equal(t, "devaddress2", labels["dev_address"])
	}
}

func collectMetric(ch chan prometheus.Metric) (*dto.Metric, map[string]string, error) {
	metric := <-ch
	dtoMetric := dto.Metric{}

	err := metric.Write(&dtoMetric)
	if err != nil {
		return nil, nil, fmt.Errorf("write metric error: %w", err)
	}

	labels := make(map[string]string, len(dtoMetric.Label))

	for _, l := range dtoMetric.Label {
		labels[*l.Name] = *l.Value
	}

	return &dtoMetric, labels, nil
}
