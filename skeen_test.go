package main

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestReceiveStartRejectsNonDestination(t *testing.T) {
	node := NewSkeen(0)
	req := Request{
		ID:   "r1",
		Type: OpPut,
		Dst:  []PartitionID{1},
		Put:  &PutPayload{Key: 1, Value: 10},
	}

	_, err := node.HandleProtocol(context.Background(), ProtocolMessage{
		Type:      MessageStart,
		From:      0,
		RequestID: req.ID,
		Request:   req,
	})
	if err == nil {
		t.Fatal("expected non-destination START to be rejected")
	}
}

func TestSubmitExecutesSinglePartitionPutAndCrossPartitionRange(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	transport := NewInMemoryTransport()
	p0 := NewSkeen(0, WithMode(ModeStrengthened))
	p1 := NewSkeen(1, WithMode(ModeStrengthened))
	transport.Register(p0)
	transport.Register(p1)

	if _, err := p0.Submit(ctx, Request{
		ID:   "put-key-0",
		Type: OpPut,
		Dst:  destinationsForPut(0),
		Put:  &PutPayload{Key: 0, Value: 42},
	}); err != nil {
		t.Fatalf("put key 0: %v", err)
	}
	if _, err := p0.Submit(ctx, Request{
		ID:   "put-key-1",
		Type: OpPut,
		Dst:  destinationsForPut(1),
		Put:  &PutPayload{Key: 1, Value: 99},
	}); err != nil {
		t.Fatalf("put key 1 through p0: %v", err)
	}

	got, err := p0.Submit(ctx, Request{
		ID:    "range",
		Type:  OpRange,
		Dst:   destinationsForRange(0, 1),
		Range: &RangePayload{Start: 0, End: 1},
	})
	if err != nil {
		t.Fatalf("range: %v", err)
	}
	want := map[int]int{0: 42, 1: 99}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("range result=%v, want %v", got, want)
	}
}

func TestSubmitExecutesAcrossFivePartitions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	router := MustRouter(5)
	nodes, _ := newInMemoryCluster(ModeStrengthened, 5, nil)
	p0 := nodes[0]

	for key := 0; key < 5; key++ {
		if _, err := p0.Submit(ctx, Request{
			ID:   "put-five-" + string(rune('a'+key)),
			Type: OpPut,
			Dst:  router.DestinationsForPut(key),
			Put:  &PutPayload{Key: key, Value: key * 10},
		}); err != nil {
			t.Fatalf("put key %d: %v", key, err)
		}
	}

	got, err := p0.Submit(ctx, Request{
		ID:    "range-all-five",
		Type:  OpRange,
		Dst:   router.DestinationsForRange(0, 4),
		Range: &RangePayload{Start: 0, End: 4},
	})
	if err != nil {
		t.Fatalf("range all five: %v", err)
	}
	want := map[int]int{0: 0, 1: 10, 2: 20, 3: 30, 4: 40}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("range result=%v, want %v", got, want)
	}
}

func TestNonOverlappingDestinationSetsDeliverIndependently(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	recorder := newDeliveryRecorder()
	nodes, _ := newInMemoryCluster(ModeStrengthened, 4, recorder.hook)
	p0 := nodes[0]

	if _, err := p0.Submit(ctx, Request{
		ID:    "range-0-1",
		Type:  OpRange,
		Dst:   []PartitionID{0, 1},
		Range: &RangePayload{Start: 0, End: 1},
	}); err != nil {
		t.Fatalf("range 0-1: %v", err)
	}
	if _, err := p0.Submit(ctx, Request{
		ID:    "range-2-3",
		Type:  OpRange,
		Dst:   []PartitionID{2, 3},
		Range: &RangePayload{Start: 2, End: 3},
	}); err != nil {
		t.Fatalf("range 2-3: %v", err)
	}

	for _, partition := range []PartitionID{0, 1} {
		if got := recorder.order(partition); !reflect.DeepEqual(got, []string{"range-0-1"}) {
			t.Fatalf("partition %d deliveries=%v, want [range-0-1]", partition, got)
		}
	}
	for _, partition := range []PartitionID{2, 3} {
		if got := recorder.order(partition); !reflect.DeepEqual(got, []string{"range-2-3"}) {
			t.Fatalf("partition %d deliveries=%v, want [range-2-3]", partition, got)
		}
	}
}

func TestOverlappingDestinationSetsPreserveOrderAtIntersection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	recorder := newDeliveryRecorder()
	nodes, _ := newInMemoryCluster(ModeOriginal, 4, recorder.hook)
	p0 := nodes[0]

	if _, err := p0.Submit(ctx, Request{
		ID:    "first",
		Type:  OpRange,
		Dst:   []PartitionID{0, 1},
		Range: &RangePayload{Start: 0, End: 1},
	}); err != nil {
		t.Fatalf("first range: %v", err)
	}
	if _, err := p0.Submit(ctx, Request{
		ID:    "second",
		Type:  OpRange,
		Dst:   []PartitionID{1, 2},
		Range: &RangePayload{Start: 1, End: 2},
	}); err != nil {
		t.Fatalf("second range: %v", err)
	}

	if got := recorder.order(1); !reflect.DeepEqual(got, []string{"first", "second"}) {
		t.Fatalf("intersection delivery order=%v, want [first second]", got)
	}
}

func TestOriginalSkeenCanViolateAtomicGlobalOrderScenario(t *testing.T) {
	nodes, recorder, transport := newManualCluster(ModeOriginal, 0, 1, 2)
	nodes[1].clock = 4
	nodes[2].clock = 2

	reqM := Request{
		ID:    "m",
		Type:  OpRange,
		Dst:   []PartitionID{0, 1},
		Range: &RangePayload{Start: 0, End: 1},
	}
	reqMPrime := Request{
		ID:    "m-prime",
		Type:  OpRange,
		Dst:   []PartitionID{0, 2},
		Range: &RangePayload{Start: 0, End: 2},
	}

	handle(t, nodes[0], startMsg(0, reqM))
	handle(t, nodes[1], startMsg(1, reqM))
	transport.deliver(t, nodes, MessageLocalTS, 0, 1, "m")

	if got := recorder.order(1); !reflect.DeepEqual(got, []string{"m"}) {
		t.Fatalf("original P1 delivery before m-prime multicast=%v, want [m]", got)
	}

	handle(t, nodes[0], startMsg(0, reqMPrime))
	handle(t, nodes[2], startMsg(2, reqMPrime))
	transport.deliver(t, nodes, MessageLocalTS, 2, 0, "m-prime")
	transport.deliver(t, nodes, MessageLocalTS, 1, 0, "m")

	if got := recorder.order(0); !reflect.DeepEqual(got, []string{"m-prime", "m"}) {
		t.Fatalf("original P0 delivery order=%v, want [m-prime m]", got)
	}
}

func TestOriginalSkeenUnsafeScenarioWithFourPartitions(t *testing.T) {
	nodes, recorder, transport := newManualCluster(ModeOriginal, 0, 1, 2, 3)
	nodes[1].clock = 4
	nodes[2].clock = 2

	reqM := Request{
		ID:    "m",
		Type:  OpRange,
		Dst:   []PartitionID{0, 1, 3},
		Range: &RangePayload{Start: 0, End: 3},
	}
	reqMPrime := Request{
		ID:    "m-prime",
		Type:  OpRange,
		Dst:   []PartitionID{0, 2},
		Range: &RangePayload{Start: 0, End: 2},
	}

	handle(t, nodes[0], startMsg(0, reqM))
	handle(t, nodes[1], startMsg(1, reqM))
	handle(t, nodes[3], startMsg(3, reqM))
	transport.deliver(t, nodes, MessageLocalTS, 0, 1, "m")
	transport.deliver(t, nodes, MessageLocalTS, 3, 1, "m")

	if got := recorder.order(1); !reflect.DeepEqual(got, []string{"m"}) {
		t.Fatalf("original P1 delivery before m-prime multicast=%v, want [m]", got)
	}

	handle(t, nodes[0], startMsg(0, reqMPrime))
	handle(t, nodes[2], startMsg(2, reqMPrime))
	transport.deliver(t, nodes, MessageLocalTS, 2, 0, "m-prime")
	transport.deliver(t, nodes, MessageLocalTS, 1, 0, "m")
	transport.deliver(t, nodes, MessageLocalTS, 3, 0, "m")

	if got := recorder.order(0); !reflect.DeepEqual(got, []string{"m-prime", "m"}) {
		t.Fatalf("original P0 delivery order=%v, want [m-prime m]", got)
	}
}

func TestStrengthenedSkeenDelaysDeliveryUntilDestinationACKs(t *testing.T) {
	nodes, recorder, transport := newManualCluster(ModeStrengthened, 0, 1, 2)
	nodes[1].clock = 4
	nodes[2].clock = 2

	reqM := Request{
		ID:    "m",
		Type:  OpRange,
		Dst:   []PartitionID{0, 1},
		Range: &RangePayload{Start: 0, End: 1},
	}
	reqMPrime := Request{
		ID:    "m-prime",
		Type:  OpRange,
		Dst:   []PartitionID{0, 2},
		Range: &RangePayload{Start: 0, End: 2},
	}

	handle(t, nodes[0], startMsg(0, reqM))
	handle(t, nodes[1], startMsg(1, reqM))
	transport.deliver(t, nodes, MessageLocalTS, 0, 1, "m")

	if got := recorder.order(1); len(got) != 0 {
		t.Fatalf("strengthened P1 delivered before ACKs: %v", got)
	}

	handle(t, nodes[0], startMsg(0, reqMPrime))
	handle(t, nodes[2], startMsg(2, reqMPrime))
	transport.deliver(t, nodes, MessageLocalTS, 2, 0, "m-prime")
	transport.deliver(t, nodes, MessageLocalTS, 1, 0, "m")
	transport.deliver(t, nodes, MessageACK, 0, 1, "m")

	if got := recorder.order(1); !reflect.DeepEqual(got, []string{"m"}) {
		t.Fatalf("strengthened P1 delivery after ACK=%v, want [m]", got)
	}
}

func TestStrengthenedSkeenDelaysUnsafeScenarioWithFourPartitions(t *testing.T) {
	nodes, recorder, transport := newManualCluster(ModeStrengthened, 0, 1, 2, 3)
	nodes[1].clock = 4
	nodes[2].clock = 2

	reqM := Request{
		ID:    "m",
		Type:  OpRange,
		Dst:   []PartitionID{0, 1, 3},
		Range: &RangePayload{Start: 0, End: 3},
	}
	reqMPrime := Request{
		ID:    "m-prime",
		Type:  OpRange,
		Dst:   []PartitionID{0, 2},
		Range: &RangePayload{Start: 0, End: 2},
	}

	handle(t, nodes[0], startMsg(0, reqM))
	handle(t, nodes[1], startMsg(1, reqM))
	handle(t, nodes[3], startMsg(3, reqM))
	transport.deliver(t, nodes, MessageLocalTS, 0, 1, "m")
	transport.deliver(t, nodes, MessageLocalTS, 3, 1, "m")

	if got := recorder.order(1); len(got) != 0 {
		t.Fatalf("strengthened P1 delivered before ACKs: %v", got)
	}

	handle(t, nodes[0], startMsg(0, reqMPrime))
	handle(t, nodes[2], startMsg(2, reqMPrime))
	transport.deliver(t, nodes, MessageLocalTS, 2, 0, "m-prime")
	transport.deliver(t, nodes, MessageLocalTS, 1, 0, "m")
	transport.deliver(t, nodes, MessageLocalTS, 3, 0, "m")
	transport.deliver(t, nodes, MessageFinalTS, 1, 3, "m")
	transport.deliver(t, nodes, MessageACK, 0, 1, "m")
	transport.deliver(t, nodes, MessageACK, 3, 1, "m")

	if got := recorder.order(1); !reflect.DeepEqual(got, []string{"m"}) {
		t.Fatalf("strengthened P1 delivery after ACK=%v, want [m]", got)
	}
}

type deliveryRecorder struct {
	mu     sync.Mutex
	orders map[PartitionID][]string
}

func newDeliveryRecorder() *deliveryRecorder {
	return &deliveryRecorder{orders: make(map[PartitionID][]string)}
}

func (r *deliveryRecorder) hook(partition PartitionID, req Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orders[partition] = append(r.orders[partition], req.ID)
}

func (r *deliveryRecorder) order(partition PartitionID) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.orders[partition]...)
}

type capturedMessage struct {
	to  PartitionID
	msg ProtocolMessage
}

type manualTransport struct {
	mu       sync.Mutex
	captured []capturedMessage
}

func (t *manualTransport) Send(_ context.Context, to PartitionID, msg ProtocolMessage) (ProtocolResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.captured = append(t.captured, capturedMessage{to: to, msg: msg})
	return ProtocolResponse{}, nil
}

func (t *manualTransport) deliver(tb testing.TB, nodes map[PartitionID]*Skeen, typ ProtocolMessageType, from, to PartitionID, requestID string) {
	tb.Helper()
	msg := t.pop(tb, typ, from, to, requestID)
	handle(tb, nodes[to], msg)
}

func (t *manualTransport) pop(tb testing.TB, typ ProtocolMessageType, from, to PartitionID, requestID string) ProtocolMessage {
	tb.Helper()
	t.mu.Lock()
	defer t.mu.Unlock()

	for i, captured := range t.captured {
		msg := captured.msg
		if captured.to == to && msg.Type == typ && msg.From == from && msg.RequestID == requestID {
			t.captured = append(t.captured[:i], t.captured[i+1:]...)
			return msg
		}
	}
	tb.Fatalf("missing captured message type=%s from=%d to=%d request=%s; captured=%v", typ, from, to, requestID, t.captured)
	return ProtocolMessage{}
}

func newManualCluster(mode ProtocolMode, ids ...PartitionID) (map[PartitionID]*Skeen, *deliveryRecorder, *manualTransport) {
	recorder := newDeliveryRecorder()
	transport := &manualTransport{}
	nodes := make(map[PartitionID]*Skeen)
	for _, id := range ids {
		nodes[id] = NewSkeen(id, WithMode(mode), WithTransport(transport), WithDeliveryHook(recorder.hook))
	}
	return nodes, recorder, transport
}

func newInMemoryCluster(mode ProtocolMode, partitionCount int, hook func(PartitionID, Request)) (map[PartitionID]*Skeen, *InMemoryTransport) {
	transport := NewInMemoryTransport()
	nodes := make(map[PartitionID]*Skeen, partitionCount)
	for i := 0; i < partitionCount; i++ {
		opts := []SkeenOption{WithMode(mode)}
		if hook != nil {
			opts = append(opts, WithDeliveryHook(hook))
		}
		node := NewSkeen(PartitionID(i), opts...)
		transport.Register(node)
		nodes[PartitionID(i)] = node
	}
	return nodes, transport
}

func startMsg(from PartitionID, req Request) ProtocolMessage {
	return ProtocolMessage{
		Type:      MessageStart,
		From:      from,
		RequestID: req.ID,
		Request:   req,
	}
}

func handle(tb testing.TB, node *Skeen, msg ProtocolMessage) {
	tb.Helper()
	if _, err := node.HandleProtocol(context.Background(), msg); err != nil {
		tb.Fatalf("handle %s from %d request %s on P%d: %v", msg.Type, msg.From, msg.RequestID, node.id, err)
	}
}
