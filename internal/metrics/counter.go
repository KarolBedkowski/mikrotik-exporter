package metrics

import "mikrotik-exporter/routeros/proto"

//
// counter.go
// Copyright (C) 2025 Karol Będkowski <Karol Będkowski@kkomp>
//
// Distributed under terms of the GPLv3 license.
//

func CountByProperty(re []*proto.Sentence, property string) map[string]int {
	counter := make(map[string]int)

	for _, re := range re {
		pool := re.Map[property]
		cnt := 1

		if count, ok := counter[pool]; ok {
			cnt = count + 1
		}

		counter[pool] = cnt
	}

	return counter
}
