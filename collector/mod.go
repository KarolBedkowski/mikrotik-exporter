package collector

//
// mod.go
// Copyright (C) 2024 Karol Będkowski <Karol Będkowski@kkomp>

type RegisteredCollector struct {
	Name        string
	Description string
	instFunc    func() routerOSCollector
}

var registeredCollectors map[string]RegisteredCollector

func registerCollector(name string, instFunc func() routerOSCollector,
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

func instanateCollector(name string) routerOSCollector {
	if f, ok := registeredCollectors[name]; ok {
		return f.instFunc()
	}

	panic("unknown collector: " + name)
}

func AvailableCollectorsNames() []string {
	res := make([]string, 0, len(registeredCollectors))

	for n := range registeredCollectors {
		res = append(res, n)
	}

	return res
}

func AvailableCollectors() []RegisteredCollector {
	res := make([]RegisteredCollector, 0, len(registeredCollectors))

	for _, k := range registeredCollectors {
		res = append(res, k)
	}

	return res
}
