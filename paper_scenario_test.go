package main

import (
	"reflect"
	"testing"
)

func TestOriginalSkeenCanReproducePaperOrderingCycle(t *testing.T) {
	transport := newScheduledCluster(ModeOriginal, 0, 1, 2)
	transport.node(1).clock = 4
	transport.node(2).clock = 2

	m := rangeRequest("m", 0, 1)
	mPrime := rangeRequest("m-prime", 0, 2)

	transport.start(t, 0, m)
	transport.start(t, 1, m)
	transport.deliverMatching(t, protocolMessage(MessageLocalTS, "m", 0, 1))

	if got := transport.deliveredOrder(1); !reflect.DeepEqual(got, []string{"m"}) {
		t.Fatalf("P1 delivered %v, want [m]", got)
	}

	// This is the paper's real-time edge: m-prime is multicast after m has
	// already been delivered at one destination.
	transport.start(t, 0, mPrime)
	transport.start(t, 2, mPrime)
	transport.deliverMatching(t, protocolMessage(MessageLocalTS, "m-prime", 2, 0))
	transport.deliverMatching(t, protocolMessage(MessageLocalTS, "m", 1, 0))

	if got := transport.deliveredOrder(0); !reflect.DeepEqual(got, []string{"m-prime", "m"}) {
		t.Fatalf("P0 delivered %v, want [m-prime m]", got)
	}

	graph := graphFromDeliveredOrders(transport.recorder, 0, 1, 2)
	graph.addEdge("m", "m-prime")
	if !graph.hasCycle() {
		t.Fatalf("expected atomic-global-order cycle from delivery edge m-prime->m plus real-time edge m->m-prime")
	}
}

func TestOriginalSkeenCannotCreatePureDeliveryOrderCycle(t *testing.T) {
	transport := newScheduledCluster(ModeOriginal, 0, 1, 2)

	m1 := rangeRequestWithDst("m1", []PartitionID{0, 1})
	m2 := rangeRequestWithDst("m2", []PartitionID{1, 2})
	m3 := rangeRequestWithDst("m3", []PartitionID{0, 2})

	for _, req := range []Request{m1, m2, m3} {
		for _, dst := range req.Dst {
			transport.start(t, dst, req)
		}
	}
	transport.drain(t)

	graph := graphFromDeliveredOrders(transport.recorder, 0, 1, 2)
	if graph.hasCycle() {
		t.Fatalf("pure delivery orders unexpectedly contain a cycle: P0=%v P1=%v P2=%v",
			transport.deliveredOrder(0), transport.deliveredOrder(1), transport.deliveredOrder(2))
	}

	// Original Skeen still assigns a single final timestamp to each message.
	// Therefore it can violate atomic global order only when the paper's
	// real-time multicast-after-delivery edge is included; it should not create
	// a delivery-only cycle such as m1<m3<m2<m1.
}

func TestStrengthenedSkeenPreventsPaperOrderingCycle(t *testing.T) {
	transport := newScheduledCluster(ModeStrengthened, 0, 1, 2)
	transport.node(1).clock = 4
	transport.node(2).clock = 2

	m := rangeRequest("m", 0, 1)
	mPrime := rangeRequest("m-prime", 0, 2)

	transport.start(t, 0, m)
	transport.start(t, 1, m)
	transport.deliverMatching(t, protocolMessage(MessageLocalTS, "m", 0, 1))

	if got := transport.deliveredOrder(1); len(got) != 0 {
		t.Fatalf("strengthened mode delivered before destination ACKs: %v", got)
	}

	transport.start(t, 0, mPrime)
	transport.start(t, 2, mPrime)
	transport.deliverMatching(t, protocolMessage(MessageLocalTS, "m-prime", 2, 0))
	transport.deliverMatching(t, protocolMessage(MessageLocalTS, "m", 1, 0))

	if got := transport.deliveredOrder(1); len(got) != 0 {
		t.Fatalf("strengthened mode delivered before P0 ACK was released: %v", got)
	}

	transport.drain(t)

	if got := transport.deliveredOrder(1); !reflect.DeepEqual(got, []string{"m"}) {
		t.Fatalf("P1 delivered %v, want [m] after ACKs are released", got)
	}

	graph := graphFromDeliveredOrders(transport.recorder, 0, 1, 2)
	if graph.hasCycle() {
		t.Fatalf("strengthened delivery graph contains a cycle: P0=%v P1=%v P2=%v",
			transport.deliveredOrder(0), transport.deliveredOrder(1), transport.deliveredOrder(2))
	}
}

func protocolMessage(typ ProtocolMessageType, requestID string, from, to PartitionID) messagePredicate {
	return andPredicates(
		byMessageType(typ),
		byRequestID(requestID),
		fromPartition(from),
		toPartition(to),
	)
}

func rangeRequest(id string, destinations ...PartitionID) Request {
	return rangeRequestWithDst(id, destinations)
}

func rangeRequestWithDst(id string, destinations []PartitionID) Request {
	return Request{
		ID:    id,
		Type:  OpRange,
		Dst:   append([]PartitionID(nil), destinations...),
		Range: &RangePayload{Start: 0, End: 2},
	}
}
