package collectors

//
// mod.go
// Copyright (C) 2024 Karol Będkowski <Karol Będkowski@kkomp>.
import (
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"mikrotik-exporter/config"

	routeros "mikrotik-exporter/routeros"

	"github.com/prometheus/client_golang/prometheus"
)

type RouterOSCollector interface {
	Describe(ch chan<- *prometheus.Desc)
	Collect(ctx *CollectorContext) error
}

type RegisteredCollector struct {
	instFunc    func() RouterOSCollector
	Name        string
	Description string
}

var registeredCollectors map[string]RegisteredCollector

func registerCollector(name string, instFunc func() RouterOSCollector,
	description string,
) {
	if registeredCollectors == nil {
		registeredCollectors = make(map[string]RegisteredCollector)
	}

	registeredCollectors[name] = RegisteredCollector{
		Name:        name,
		Description: description,
		instFunc:    instFunc,
	}
}

func InstanateCollector(name string) RouterOSCollector {
	if f, ok := registeredCollectors[name]; ok {
		return f.instFunc()
	}

	panic("unknown collector: " + name)
}

func AvailableCollectorsNames() []string {
	return slices.Collect(maps.Keys(registeredCollectors))
}

func AvailableCollectors() []RegisteredCollector {
	return slices.Collect(maps.Values(registeredCollectors))
}

type CollectorContext struct {
	ch        chan<- prometheus.Metric
	device    *config.Device
	client    *routeros.Client
	collector string

	logger *slog.Logger

	labels []string
}

func NewCollectorContext(ch chan<- prometheus.Metric, device *config.Device, client *routeros.Client,
	collector string, logger *slog.Logger,
) CollectorContext {
	return CollectorContext{
		ch:        ch,
		device:    device,
		client:    client,
		collector: collector,
		labels:    []string{device.Name, device.Address},
		logger:    logger,
	}
}

func (c CollectorContext) withLabels(labels ...string) CollectorContext {
	return CollectorContext{
		ch:        c.ch,
		device:    c.device,
		client:    c.client,
		collector: c.collector,
		labels:    append([]string{c.device.Name, c.device.Address}, labels...),
		logger:    c.logger,
	}
}

func (c CollectorContext) withLabelsFromMap(values map[string]string, labelName ...string) CollectorContext {
	labels := []string{c.device.Name, c.device.Address}
	for _, n := range labelName {
		labels = append(labels, values[n])
	}

	return CollectorContext{
		ch:        c.ch,
		device:    c.device,
		client:    c.client,
		collector: c.collector,
		labels:    labels,
		logger:    c.logger,
	}
}

func (c CollectorContext) appendLabelsFromMap(values map[string]string, labelName ...string) CollectorContext {
	labels := c.labels
	for _, n := range labelName {
		labels = append(labels, values[n])
	}

	return CollectorContext{
		ch:        c.ch,
		device:    c.device,
		client:    c.client,
		collector: c.collector,
		labels:    labels,
		logger:    c.logger,
	}
}

//-----------------

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
