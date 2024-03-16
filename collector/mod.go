package collector

//
// mod.go
// Copyright (C) 2024 Karol Będkowski <Karol Będkowski@kkomp>

var registeredCollectors map[string]func() routerOSCollector

func registerCollector(name string, f func() routerOSCollector) {
	if registeredCollectors == nil {
		registeredCollectors = make(map[string]func() routerOSCollector)
	}

	registeredCollectors[name] = f
}

func instanateCollector(name string) routerOSCollector {
	if f, ok := registeredCollectors[name]; ok {
		return f()
	}

	panic("unknown collector: " + name)
}

func AvailableCollectors() []string {
	res := make([]string, 0, len(registeredCollectors))

	for k := range registeredCollectors {
		res = append(res, k)
	}

	return res
}
