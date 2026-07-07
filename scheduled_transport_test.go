package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

type scheduledMessage struct {
	to  PartitionID
	msg ProtocolMessage
}

type scheduledTransport struct {
	mu       sync.Mutex
	nodes    map[PartitionID]*Skeen
	queued   []scheduledMessage
	recorder *deliveryRecorder
}

type messagePredicate func(scheduledMessage) bool

func newScheduledCluster(mode ProtocolMode, ids ...PartitionID) *scheduledTransport {
	recorder := newDeliveryRecorder()
	transport := &scheduledTransport{
		nodes:    make(map[PartitionID]*Skeen, len(ids)),
		recorder: recorder,
	}
	for _, id := range ids {
		node := NewSkeen(id, WithMode(mode), WithTransport(transport), WithDeliveryHook(recorder.hook))
		transport.nodes[id] = node
	}
	return transport
}

func (t *scheduledTransport) Send(_ context.Context, to PartitionID, msg ProtocolMessage) (ProtocolResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.queued = append(t.queued, scheduledMessage{to: to, msg: msg})
	return ProtocolResponse{}, nil
}

func (t *scheduledTransport) node(id PartitionID) *Skeen {
	return t.nodes[id]
}

func (t *scheduledTransport) start(tb testing.TB, to PartitionID, req Request) {
	tb.Helper()
	t.handle(tb, to, ProtocolMessage{
		Type:      MessageStart,
		From:      to,
		RequestID: req.ID,
		Request:   req,
	})
}

func (t *scheduledTransport) deliverNext(tb testing.TB) scheduledMessage {
	tb.Helper()
	msg, ok := t.popFirst(func(scheduledMessage) bool { return true })
	if !ok {
		tb.Fatal("no queued messages")
	}
	t.handle(tb, msg.to, msg.msg)
	return msg
}

func (t *scheduledTransport) deliverMatching(tb testing.TB, predicate messagePredicate) scheduledMessage {
	tb.Helper()
	msg, ok := t.popFirst(predicate)
	if !ok {
		tb.Fatalf("no queued message matched predicate; queued=%s", t.describeQueued())
	}
	t.handle(tb, msg.to, msg.msg)
	return msg
}

func (t *scheduledTransport) deliverAllMatching(tb testing.TB, predicate messagePredicate) int {
	tb.Helper()
	delivered := 0
	for {
		msg, ok := t.popFirst(predicate)
		if !ok {
			return delivered
		}
		t.handle(tb, msg.to, msg.msg)
		delivered++
	}
}

func (t *scheduledTransport) drain(tb testing.TB) {
	tb.Helper()
	for i := 0; i < 1000; i++ {
		t.mu.Lock()
		empty := len(t.queued) == 0
		t.mu.Unlock()
		if empty {
			return
		}
		t.deliverNext(tb)
	}
	tb.Fatalf("scheduled transport did not drain; queued=%s", t.describeQueued())
}

func (t *scheduledTransport) queuedMessages() []scheduledMessage {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]scheduledMessage(nil), t.queued...)
}

func (t *scheduledTransport) deliveredOrder(partition PartitionID) []string {
	return t.recorder.order(partition)
}

func (t *scheduledTransport) popFirst(predicate messagePredicate) (scheduledMessage, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i, msg := range t.queued {
		if predicate(msg) {
			t.queued = append(t.queued[:i], t.queued[i+1:]...)
			return msg, true
		}
	}
	return scheduledMessage{}, false
}

func (t *scheduledTransport) handle(tb testing.TB, to PartitionID, msg ProtocolMessage) {
	tb.Helper()
	node := t.nodes[to]
	if node == nil {
		tb.Fatalf("unknown scheduled partition %d", to)
	}
	if _, err := node.HandleProtocol(context.Background(), msg); err != nil {
		tb.Fatalf("handle %s from %d to %d request %s: %v", msg.Type, msg.From, to, msg.RequestID, err)
	}
}

func (t *scheduledTransport) describeQueued() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.queued) == 0 {
		return "[]"
	}
	out := "["
	for i, queued := range t.queued {
		if i > 0 {
			out += " "
		}
		out += fmt.Sprintf("{type=%s request=%s from=%d to=%d}", queued.msg.Type, queued.msg.RequestID, queued.msg.From, queued.to)
	}
	out += "]"
	return out
}

func byRequestID(requestID string) messagePredicate {
	return func(msg scheduledMessage) bool {
		return msg.msg.RequestID == requestID
	}
}

func byMessageType(typ ProtocolMessageType) messagePredicate {
	return func(msg scheduledMessage) bool {
		return msg.msg.Type == typ
	}
}

func fromPartition(from PartitionID) messagePredicate {
	return func(msg scheduledMessage) bool {
		return msg.msg.From == from
	}
}

func toPartition(to PartitionID) messagePredicate {
	return func(msg scheduledMessage) bool {
		return msg.to == to
	}
}

func destinationSetContains(partition PartitionID) messagePredicate {
	return func(msg scheduledMessage) bool {
		return containsPartition(msg.msg.Request.Dst, partition)
	}
}

func andPredicates(predicates ...messagePredicate) messagePredicate {
	return func(msg scheduledMessage) bool {
		for _, predicate := range predicates {
			if !predicate(msg) {
				return false
			}
		}
		return true
	}
}

type orderGraph map[string]map[string]bool

func graphFromDeliveredOrders(recorder *deliveryRecorder, partitions ...PartitionID) orderGraph {
	graph := make(orderGraph)
	for _, partition := range partitions {
		order := recorder.order(partition)
		for i := 0; i < len(order); i++ {
			for j := i + 1; j < len(order); j++ {
				graph.addEdge(order[i], order[j])
			}
		}
	}
	return graph
}

func (g orderGraph) addEdge(from, to string) {
	if g[from] == nil {
		g[from] = make(map[string]bool)
	}
	g[from][to] = true
	if g[to] == nil {
		g[to] = make(map[string]bool)
	}
}

func (g orderGraph) hasCycle() bool {
	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)
	state := make(map[string]int, len(g))
	var visit func(string) bool
	visit = func(node string) bool {
		switch state[node] {
		case visiting:
			return true
		case visited:
			return false
		}
		state[node] = visiting
		for next := range g[node] {
			if visit(next) {
				return true
			}
		}
		state[node] = visited
		return false
	}
	for node := range g {
		if visit(node) {
			return true
		}
	}
	return false
}
