package main

import (
	"context"
	"time"
)

type latencyTransport struct {
	base       *InMemoryTransport
	fixedDelay time.Duration
	typeDelay  map[ProtocolMessageType]time.Duration
}

func newLatencyCluster(mode ProtocolMode, partitionCount int, fixedDelay time.Duration, typeDelay map[ProtocolMessageType]time.Duration) (map[PartitionID]*Skeen, *latencyTransport) {
	base := NewInMemoryTransport()
	transport := &latencyTransport{
		base:       base,
		fixedDelay: fixedDelay,
		typeDelay:  typeDelay,
	}

	nodes := make(map[PartitionID]*Skeen, partitionCount)
	for i := 0; i < partitionCount; i++ {
		node := NewSkeen(PartitionID(i), WithMode(mode))
		base.Register(node)
		node.SetTransport(transport)
		nodes[PartitionID(i)] = node
	}
	return nodes, transport
}

func (t *latencyTransport) Send(ctx context.Context, to PartitionID, msg ProtocolMessage) (ProtocolResponse, error) {
	delay := t.fixedDelay
	if typeDelay, ok := t.typeDelay[msg.Type]; ok {
		delay += typeDelay
	}
	if delay > 0 {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ProtocolResponse{}, ctx.Err()
		case <-timer.C:
		}
	}
	return t.base.Send(ctx, to, msg)
}
