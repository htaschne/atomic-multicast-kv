package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"testing"
)

func BenchmarkOriginalSinglePartitionPut(b *testing.B) {
	benchmarkSinglePartitionPut(b, ModeOriginal)
}

func BenchmarkStrengthenedSinglePartitionPut(b *testing.B) {
	benchmarkSinglePartitionPut(b, ModeStrengthened)
}

func BenchmarkOriginalSinglePartitionRange(b *testing.B) {
	benchmarkSinglePartitionRange(b, ModeOriginal)
}

func BenchmarkStrengthenedSinglePartitionRange(b *testing.B) {
	benchmarkSinglePartitionRange(b, ModeStrengthened)
}

func BenchmarkOriginalCrossPartitionRange(b *testing.B) {
	benchmarkCrossPartitionRange(b, ModeOriginal)
}

func BenchmarkStrengthenedCrossPartitionRange(b *testing.B) {
	benchmarkCrossPartitionRange(b, ModeStrengthened)
}

func benchmarkSinglePartitionPut(b *testing.B, mode ProtocolMode) {
	discardLogs(b)
	p0, _, _ := benchmarkCluster(mode)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p0.Submit(ctx, Request{
			ID:   fmt.Sprintf("put-%s-%d", mode, i),
			Type: OpPut,
			Dst:  []PartitionID{0},
			Put:  &PutPayload{Key: 0, Value: i},
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkSinglePartitionRange(b *testing.B, mode ProtocolMode) {
	discardLogs(b)
	p0, _, _ := benchmarkCluster(mode)
	ctx := context.Background()
	if _, err := p0.Submit(ctx, Request{
		ID:   "seed-put",
		Type: OpPut,
		Dst:  []PartitionID{0},
		Put:  &PutPayload{Key: 0, Value: 42},
	}); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p0.Submit(ctx, Request{
			ID:    fmt.Sprintf("range-%s-%d", mode, i),
			Type:  OpRange,
			Dst:   []PartitionID{0},
			Range: &RangePayload{Start: 0, End: 0},
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkCrossPartitionRange(b *testing.B, mode ProtocolMode) {
	discardLogs(b)
	p0, _, _ := benchmarkCluster(mode)
	ctx := context.Background()
	if _, err := p0.Submit(ctx, Request{
		ID:   "seed-even",
		Type: OpPut,
		Dst:  []PartitionID{0},
		Put:  &PutPayload{Key: 0, Value: 42},
	}); err != nil {
		b.Fatal(err)
	}
	if _, err := p0.Submit(ctx, Request{
		ID:   "seed-odd",
		Type: OpPut,
		Dst:  []PartitionID{1},
		Put:  &PutPayload{Key: 1, Value: 99},
	}); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p0.Submit(ctx, Request{
			ID:    fmt.Sprintf("cross-range-%s-%d", mode, i),
			Type:  OpRange,
			Dst:   []PartitionID{0, 1},
			Range: &RangePayload{Start: 0, End: 1},
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkCluster(mode ProtocolMode) (*Skeen, *Skeen, *InMemoryTransport) {
	transport := NewInMemoryTransport()
	p0 := NewSkeen(0, WithMode(mode))
	p1 := NewSkeen(1, WithMode(mode))
	transport.Register(p0)
	transport.Register(p1)
	return p0, p1, transport
}

func discardLogs(b *testing.B) {
	original := log.Writer()
	log.SetOutput(io.Discard)
	b.Cleanup(func() {
		log.SetOutput(original)
	})
}
