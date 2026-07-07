package main

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func BenchmarkSkeenDestinationOverhead(b *testing.B) {
	for _, partitionCount := range []int{2, 3, 5} {
		for _, mode := range []ProtocolMode{ModeOriginal, ModeStrengthened} {
			for _, destinationCount := range destinationCounts(partitionCount) {
				name := fmt.Sprintf("N=%d/mode=%s/dst=%d", partitionCount, mode, destinationCount)
				b.Run(name, func(b *testing.B) {
					benchmarkRangeDestinationCount(b, mode, partitionCount, destinationCount)
				})
			}
		}
	}
}

func BenchmarkOriginalSinglePartitionPut(b *testing.B) {
	benchmarkSinglePartitionPut(b, ModeOriginal, 2)
}

func BenchmarkStrengthenedSinglePartitionPut(b *testing.B) {
	benchmarkSinglePartitionPut(b, ModeStrengthened, 2)
}

func BenchmarkSkeenArtificialLatency(b *testing.B) {
	for _, mode := range []ProtocolMode{ModeOriginal, ModeStrengthened} {
		for _, destinationCount := range []int{1, 2, 3} {
			for _, delay := range []time.Duration{0, time.Millisecond, 5 * time.Millisecond, 10 * time.Millisecond} {
				name := fmt.Sprintf("N=3/mode=%s/dst=%d/delay=%s", mode, destinationCount, delay)
				b.Run(name, func(b *testing.B) {
					benchmarkRangeWithLatency(b, mode, 3, destinationCount, delay, 0)
				})
			}
		}
	}
}

func BenchmarkSkeenAckLatency(b *testing.B) {
	for _, ackDelay := range []time.Duration{0, time.Millisecond, 5 * time.Millisecond, 10 * time.Millisecond} {
		name := fmt.Sprintf("N=3/mode=%s/dst=3/ackDelay=%s", ModeStrengthened, ackDelay)
		b.Run(name, func(b *testing.B) {
			benchmarkRangeWithLatency(b, ModeStrengthened, 3, 3, 0, ackDelay)
		})
	}
}

func benchmarkRangeDestinationCount(b *testing.B, mode ProtocolMode, partitionCount, destinationCount int) {
	SetProtocolLogging(false)
	nodes, _ := benchmarkCluster(mode, partitionCount)
	p0 := nodes[0]
	ctx := context.Background()
	dst := firstDestinations(destinationCount)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p0.Submit(ctx, Request{
			ID:    fmt.Sprintf("range-%s-n%d-d%d-%d", mode, partitionCount, destinationCount, i),
			Type:  OpRange,
			Dst:   dst,
			Range: &RangePayload{Start: 0, End: partitionCount - 1},
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkRangeWithLatency(b *testing.B, mode ProtocolMode, partitionCount, destinationCount int, fixedDelay, ackDelay time.Duration) {
	SetProtocolLogging(false)
	typeDelay := map[ProtocolMessageType]time.Duration{}
	if ackDelay > 0 {
		typeDelay[MessageACK] = ackDelay
	}
	nodes, _ := newLatencyCluster(mode, partitionCount, fixedDelay, typeDelay)
	p0 := nodes[0]
	ctx := context.Background()
	dst := firstDestinations(destinationCount)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p0.Submit(ctx, Request{
			ID:    fmt.Sprintf("latency-range-%s-n%d-d%d-delay%d-ack%d-%d", mode, partitionCount, destinationCount, fixedDelay.Milliseconds(), ackDelay.Milliseconds(), i),
			Type:  OpRange,
			Dst:   dst,
			Range: &RangePayload{Start: 0, End: partitionCount - 1},
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkSinglePartitionPut(b *testing.B, mode ProtocolMode, partitionCount int) {
	SetProtocolLogging(false)
	nodes, _ := benchmarkCluster(mode, partitionCount)
	p0 := nodes[0]
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p0.Submit(ctx, Request{
			ID:   fmt.Sprintf("put-%s-n%d-%d", mode, partitionCount, i),
			Type: OpPut,
			Dst:  []PartitionID{0},
			Put:  &PutPayload{Key: 0, Value: i},
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkCluster(mode ProtocolMode, partitionCount int) (map[PartitionID]*Skeen, *InMemoryTransport) {
	return newInMemoryCluster(mode, partitionCount, nil)
}

func destinationCounts(partitionCount int) []int {
	counts := []int{1}
	for _, count := range []int{2, 3, partitionCount} {
		if count <= partitionCount && !containsInt(counts, count) {
			counts = append(counts, count)
		}
	}
	return counts
}

func firstDestinations(count int) []PartitionID {
	dst := make([]PartitionID, 0, count)
	for i := 0; i < count; i++ {
		dst = append(dst, PartitionID(i))
	}
	return dst
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
