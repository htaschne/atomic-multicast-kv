package main

import "sync"

var (
	storage = make(map[int]int)
	mu      sync.Mutex
)

func put(key, value int) {
	mu.Lock()
	defer mu.Unlock()

	storage[key] = value
}

func getRange(start, end int) map[int]int {
	mu.Lock()
	defer mu.Unlock()

	res := make(map[int]int)
	for i := start; i <= end; i++ {
		if value, ok := storage[i]; ok {
			res[i] = value
		}
	}
	return res
}
