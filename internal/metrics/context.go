package metrics

//
// mod.go
// Copyright (C) 2024 Karol Będkowski <Karol Będkowski@kkomp>.
import (
	"fmt"
	"log/slog"
	"slices"

	"mikrotik-exporter/internal/config"

	"github.com/prometheus/client_golang/prometheus"

	routeros "mikrotik-exporter/routeros"
)

// ----------------------------------------------------------------------------

type ROClient interface {
	Run(sentence ...string) (*routeros.Reply, error)
}

// ----------------------------------------------------------------------------

type CollectorContext struct {
	Ch        chan<- prometheus.Metric
	Device    *config.Device
	Client    ROClient
	collector string

	Logger     *slog.Logger
	FeatureCfg config.FeatureConf

	Labels []string
}

func NewCollectorContext(ch chan<- prometheus.Metric, device *config.Device, client ROClient,
	collector string, logger *slog.Logger, featureCfg config.FeatureConf,
) CollectorContext {
	return CollectorContext{
		Ch:         ch,
		Device:     device,
		Client:     client,
		collector:  collector,
		Labels:     []string{device.Name, device.Address},
		Logger:     logger,
		FeatureCfg: featureCfg,
	}
}

func (c CollectorContext) WithLabels(labels ...string) CollectorContext {
	return CollectorContext{
		Ch:         c.Ch,
		Device:     c.Device,
		Client:     c.Client,
		collector:  c.collector,
		Labels:     append([]string{c.Device.Name, c.Device.Address}, labels...),
		Logger:     c.Logger,
		FeatureCfg: c.FeatureCfg,
	}
}

func (c CollectorContext) WithLabelsFromMap(values map[string]string, labelName ...string) CollectorContext {
	labels := []string{c.Device.Name, c.Device.Address}
	for _, n := range labelName {
		labels = append(labels, values[n])
	}

	return CollectorContext{
		Ch:         c.Ch,
		Device:     c.Device,
		Client:     c.Client,
		collector:  c.collector,
		Labels:     labels,
		Logger:     c.Logger,
		FeatureCfg: c.FeatureCfg,
	}
}

func (c CollectorContext) AppendLabelsFromMap(values map[string]string, labelName ...string) CollectorContext {
	labels := slices.Clone(c.Labels)
	for _, n := range labelName {
		labels = append(labels, values[n])
	}

	return CollectorContext{
		Ch:         c.Ch,
		Device:     c.Device,
		Client:     c.Client,
		collector:  c.collector,
		Labels:     labels,
		Logger:     c.Logger,
		FeatureCfg: c.FeatureCfg,
	}
}

func (c CollectorContext) Run(sentence ...string) (*routeros.Reply, error) {
	reply, err := c.Client.Run(sentence...)
	if err != nil {
		return nil, fmt.Errorf("client run error: %w", err)
	}

	return reply, nil
}
