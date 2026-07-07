package main

import "testing"

func TestRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		req     Request
		wantErr bool
	}{
		{
			name: "valid put",
			req: Request{
				ID:   "r1",
				Type: OpPut,
				Dst:  []PartitionID{0},
				Put:  &PutPayload{Key: 0, Value: 10},
			},
		},
		{
			name: "valid range",
			req: Request{
				ID:    "r2",
				Type:  OpRange,
				Dst:   []PartitionID{0, 1},
				Range: &RangePayload{Start: 0, End: 2},
			},
		},
		{
			name:    "missing id",
			req:     Request{Type: OpPut, Dst: []PartitionID{0}, Put: &PutPayload{}},
			wantErr: true,
		},
		{
			name:    "missing destination",
			req:     Request{ID: "r1", Type: OpPut, Put: &PutPayload{}},
			wantErr: true,
		},
		{
			name:    "duplicate destination",
			req:     Request{ID: "r1", Type: OpRange, Dst: []PartitionID{0, 0}, Range: &RangePayload{}},
			wantErr: true,
		},
		{
			name:    "negative destination",
			req:     Request{ID: "r1", Type: OpRange, Dst: []PartitionID{-1}, Range: &RangePayload{}},
			wantErr: true,
		},
		{
			name: "put must have one destination",
			req: Request{
				ID:   "r1",
				Type: OpPut,
				Dst:  []PartitionID{0, 1},
				Put:  &PutPayload{},
			},
			wantErr: true,
		},
		{
			name: "range start after end",
			req: Request{
				ID:    "r1",
				Type:  OpRange,
				Dst:   []PartitionID{0},
				Range: &RangePayload{Start: 2, End: 1},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestTimestampOrdering(t *testing.T) {
	if !((Timestamp{Clock: 1, Partition: 1}).Less(Timestamp{Clock: 2, Partition: 0})) {
		t.Fatal("lower logical clock should sort first")
	}
	if !((Timestamp{Clock: 1, Partition: 0}).Less(Timestamp{Clock: 1, Partition: 1})) {
		t.Fatal("partition id should break timestamp ties")
	}
}
