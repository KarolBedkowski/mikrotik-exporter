package metrics

//
// mod.go
// Copyright (C) 2024 Karol Będkowski <Karol Będkowski@kkomp>.
import (
	"log/slog"
	"slices"

	"mikrotik-exporter/internal/config"
	routeros "mikrotik-exporter/routeros"

	"github.com/prometheus/client_golang/prometheus"
)

// ----------------------------------------------------------------------------

type CollectorContext struct {
	Ch        chan<- prometheus.Metric
	Device    *config.Device
	Client    *routeros.Client
	collector string

	Logger *slog.Logger

	Labels []string
}

func NewCollectorContext(ch chan<- prometheus.Metric, device *config.Device, client *routeros.Client,
	collector string, logger *slog.Logger,
) CollectorContext {
	return CollectorContext{
		Ch:        ch,
		Device:    device,
		Client:    client,
		collector: collector,
		Labels:    []string{device.Name, device.Address},
		Logger:    logger,
	}
}

func (c CollectorContext) WithLabels(labels ...string) CollectorContext {
	return CollectorContext{
		Ch:        c.Ch,
		Device:    c.Device,
		Client:    c.Client,
		collector: c.collector,
		Labels:    append([]string{c.Device.Name, c.Device.Address}, labels...),
		Logger:    c.Logger,
	}
}

func (c CollectorContext) WithLabelsFromMap(values map[string]string, labelName ...string) CollectorContext {
	labels := []string{c.Device.Name, c.Device.Address}
	for _, n := range labelName {
		labels = append(labels, values[n])
	}

	return CollectorContext{
		Ch:        c.Ch,
		Device:    c.Device,
		Client:    c.Client,
		collector: c.collector,
		Labels:    labels,
		Logger:    c.Logger,
	}
}

func (c CollectorContext) AppendLabelsFromMap(values map[string]string, labelName ...string) CollectorContext {
	labels := slices.Clone(c.Labels)
	for _, n := range labelName {
		labels = append(labels, values[n])
	}

	return CollectorContext{
		Ch:        c.Ch,
		Device:    c.Device,
		Client:    c.Client,
		collector: c.collector,
		Labels:    labels,
		Logger:    c.Logger,
	}
}

// ----------------------------------------------------------------------------
