package main

import (
	"reflect"
	"testing"
)

func TestKVStorePutRange(t *testing.T) {
	store := NewKVStore()
	store.Put(0, 42)
	store.Put(2, 84)
	store.Put(3, 99)

	got := store.Range(0, 2)
	want := map[int]int{0: 42, 2: 84}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Range(0,2)=%v, want %v", got, want)
	}
}
