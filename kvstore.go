package main

import "sync"

type KVStore struct {
	mu      sync.Mutex
	storage map[int]int
}

func NewKVStore() *KVStore {
	return &KVStore{storage: make(map[int]int)}
}

func (s *KVStore) Put(key, value int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.storage[key] = value
}

func (s *KVStore) Range(start, end int) map[int]int {
	s.mu.Lock()
	defer s.mu.Unlock()

	res := make(map[int]int)
	for i := start; i <= end; i++ {
		if value, ok := s.storage[i]; ok {
			res[i] = value
		}
	}
	return res
}

var (
	defaultStore = NewKVStore()
)

func put(key, value int) {
	defaultStore.Put(key, value)
}

func getRange(start, end int) map[int]int {
	return defaultStore.Range(start, end)
}
