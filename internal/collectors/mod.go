package collectors

//
// mod.go
// Copyright (C) 2024 Karol Będkowski <Karol Będkowski@kkomp>.
import (
	"fmt"
	"maps"
	"slices"

	"mikrotik-exporter/internal/metrics"

	"github.com/prometheus/client_golang/prometheus"

	routeros "mikrotik-exporter/routeros"
)

type RouterOSCollector interface {
	Describe(ch chan<- *prometheus.Desc)
	Collect(ctx *metrics.CollectorContext) error
}

// ----------------------------------------------------------------------------

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

// ----------------------------------------------------------------------------

type UnexpectedResponseError struct {
	msg   string
	reply *routeros.Reply
}

func (u UnexpectedResponseError) Error() string {
	return fmt.Sprintf("%s: %v", u.msg, u.reply)
}
