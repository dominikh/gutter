// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"runtime"
	"sync"
)

func distribute[S ~[]E, E any](items S, limit int, fn func(group int, step int, subitems S) error) error {
	if len(items) == 0 {
		return nil
	}

	if limit <= 0 {
		limit = runtime.GOMAXPROCS(0)
	}

	if limit > len(items) {
		limit = len(items)
	}

	step := len(items) / limit
	var muGerr sync.Mutex
	var gerr error
	var wg sync.WaitGroup
	wg.Add(limit)
	for g := range limit {
		go func(g int) {
			defer wg.Done()
			var subset S
			if g < limit-1 {
				subset = items[g*step : (g+1)*step]
			} else {
				subset = items[g*step:]
			}
			if err := fn(g, step, subset); err != nil {
				muGerr.Lock()
				if gerr == nil {
					gerr = err
				}
				muGerr.Unlock()
			}
		}(g)
	}
	wg.Wait()
	return gerr
}
