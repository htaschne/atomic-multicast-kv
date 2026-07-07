package main

import (
	"reflect"
	"testing"
)

func TestRouting(t *testing.T) {
	if got := partitionForKey(2); got != 0 {
		t.Fatalf("partitionForKey(2)=%d, want 0", got)
	}
	if got := partitionForKey(3); got != 1 {
		t.Fatalf("partitionForKey(3)=%d, want 1", got)
	}
	if got := destinationsForPut(4); !reflect.DeepEqual(got, []PartitionID{0}) {
		t.Fatalf("destinationsForPut(4)=%v, want [0]", got)
	}
	if got := destinationsForPut(5); !reflect.DeepEqual(got, []PartitionID{1}) {
		t.Fatalf("destinationsForPut(5)=%v, want [1]", got)
	}
	if got := destinationsForRange(0, 0); !reflect.DeepEqual(got, []PartitionID{0}) {
		t.Fatalf("destinationsForRange(0,0)=%v, want [0]", got)
	}
	if got := destinationsForRange(1, 1); !reflect.DeepEqual(got, []PartitionID{1}) {
		t.Fatalf("destinationsForRange(1,1)=%v, want [1]", got)
	}
	if got := destinationsForRange(0, 1); !reflect.DeepEqual(got, []PartitionID{0, 1}) {
		t.Fatalf("destinationsForRange(0,1)=%v, want [0 1]", got)
	}
}

func TestRouterWithNPartitions(t *testing.T) {
	router := MustRouter(5)

	if got := router.PartitionForKey(7); got != 2 {
		t.Fatalf("PartitionForKey(7)=%d, want 2", got)
	}
	if got := router.PartitionForKey(-1); got != 4 {
		t.Fatalf("PartitionForKey(-1)=%d, want 4", got)
	}
	if got := router.DestinationsForPut(8); !reflect.DeepEqual(got, []PartitionID{3}) {
		t.Fatalf("DestinationsForPut(8)=%v, want [3]", got)
	}
	if got := router.DestinationsForRange(0, 2); !reflect.DeepEqual(got, []PartitionID{0, 1, 2}) {
		t.Fatalf("DestinationsForRange(0,2)=%v, want [0 1 2]", got)
	}
	if got := router.DestinationsForRange(0, 8); !reflect.DeepEqual(got, []PartitionID{0, 1, 2, 3, 4}) {
		t.Fatalf("DestinationsForRange(0,8)=%v, want all partitions", got)
	}
	if got := router.DestinationsForRange(3, 3); !reflect.DeepEqual(got, []PartitionID{3}) {
		t.Fatalf("DestinationsForRange(3,3)=%v, want [3]", got)
	}
}

func TestNewRouterRejectsInvalidPartitionCount(t *testing.T) {
	if _, err := NewRouter(0); err == nil {
		t.Fatal("expected invalid partition count error")
	}
}
