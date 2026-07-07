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
