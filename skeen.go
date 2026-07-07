package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"
)

var requestCounter uint64

type OpType string

const (
	OpPut   OpType = "PUT"
	OpRange OpType = "RANGE"
)

type PartitionID int

func newRequestID(partitionID PartitionID) string {
	id := atomic.AddUint64(&requestCounter, 1)
	return fmt.Sprintf("p%d-%d", partitionID, id)
}

type PutPayload struct {
	Key   int
	Value int
}

type RangePayload struct {
	Start int
	End   int
}

type Request struct {
	ID    string
	Type  OpType
	Dst   []PartitionID
	Put   *PutPayload
	Range *RangePayload
}

func (r Request) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("missing request id")
	}
	if len(r.Dst) == 0 {
		return fmt.Errorf("missing destination set")
	}

	switch r.Type {
	case OpPut:
		if r.Put == nil {
			return fmt.Errorf("missing put payload")
		}
		if r.Range != nil {
			return fmt.Errorf("put request cannot contain range payload")
		}
		if len(r.Dst) != 1 {
			return fmt.Errorf("put request must target exactly one partition")
		}
	case OpRange:
		if r.Range == nil {
			return fmt.Errorf("missing range payload")
		}
		if r.Put != nil {
			return fmt.Errorf("range request cannot contain put payload")
		}
		if r.Range.Start > r.Range.End {
			return fmt.Errorf("invalid range: start > end")
		}
	default:
		return fmt.Errorf("unknown op type: %s", r.Type)
	}

	return nil
}

type ProtocolMode string

const (
	ModeOriginal     ProtocolMode = "original"
	ModeStrengthened ProtocolMode = "strengthened"
)

func parseProtocolMode(mode string) (ProtocolMode, error) {
	switch ProtocolMode(mode) {
	case "", ModeOriginal:
		return ModeOriginal, nil
	case ModeStrengthened:
		return ModeStrengthened, nil
	default:
		return "", fmt.Errorf("unknown protocol mode: %s", mode)
	}
}

type Timestamp struct {
	Clock     uint64      `json:"clock"`
	Partition PartitionID `json:"partition"`
}

func (t Timestamp) Less(other Timestamp) bool {
	if t.Clock != other.Clock {
		return t.Clock < other.Clock
	}
	return t.Partition < other.Partition
}

func (t Timestamp) Equal(other Timestamp) bool {
	return t.Clock == other.Clock && t.Partition == other.Partition
}

type ProtocolMessageType string

const (
	MessageStart   ProtocolMessageType = "START"
	MessageLocalTS ProtocolMessageType = "LOCAL_TS"
	MessageFinalTS ProtocolMessageType = "FINAL_TS"
	MessageACK     ProtocolMessageType = "ACK"
)

type ProtocolMessage struct {
	Type      ProtocolMessageType `json:"type"`
	From      PartitionID         `json:"from"`
	RequestID string              `json:"request_id"`
	Request   Request             `json:"request"`
	Timestamp Timestamp           `json:"timestamp"`
}

type ProtocolResponse struct {
	Result map[int]int `json:"result,omitempty"`
}

type Transport interface {
	Send(ctx context.Context, to PartitionID, msg ProtocolMessage) (ProtocolResponse, error)
}

type requestState struct {
	req       Request
	local     *Timestamp
	proposals map[PartitionID]Timestamp
	final     *Timestamp
	acks      map[PartitionID]bool
	ackSent   bool
	delivered bool
	result    map[int]int
	done      chan struct{}
}

type Skeen struct {
	id        PartitionID
	mode      ProtocolMode
	store     *KVStore
	transport Transport
	onDeliver func(PartitionID, Request)

	mu     sync.Mutex
	clock  uint64
	states map[string]*requestState
}

type SkeenOption func(*Skeen)

func WithMode(mode ProtocolMode) SkeenOption {
	return func(s *Skeen) {
		s.mode = mode
	}
}

func WithStore(store *KVStore) SkeenOption {
	return func(s *Skeen) {
		s.store = store
	}
}

func WithTransport(transport Transport) SkeenOption {
	return func(s *Skeen) {
		s.transport = transport
	}
}

func WithDeliveryHook(hook func(PartitionID, Request)) SkeenOption {
	return func(s *Skeen) {
		s.onDeliver = hook
	}
}

func NewSkeen(id PartitionID, opts ...SkeenOption) *Skeen {
	s := &Skeen{
		id:     id,
		mode:   ModeOriginal,
		store:  NewKVStore(),
		states: make(map[string]*requestState),
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.store == nil {
		s.store = NewKVStore()
	}
	return s
}

func (s *Skeen) SetTransport(transport Transport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transport = transport
}

func (s *Skeen) Submit(ctx context.Context, req Request) (map[int]int, error) {
	log.Printf("[P%d] SUBMIT id=%s type=%s dst=%v", s.id, req.ID, req.Type, req.Dst)
	if err := req.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	transport := s.transport
	s.mu.Unlock()
	if transport == nil {
		return nil, fmt.Errorf("missing transport")
	}

	type response struct {
		result ProtocolResponse
		err    error
	}

	responses := make(chan response, len(req.Dst))
	for _, dst := range req.Dst {
		dst := dst
		go func() {
			result, err := transport.Send(ctx, dst, ProtocolMessage{
				Type:      MessageStart,
				From:      s.id,
				RequestID: req.ID,
				Request:   req,
			})
			responses <- response{result: result, err: err}
		}()
	}

	merged := make(map[int]int)
	for range req.Dst {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case resp := <-responses:
			if resp.err != nil {
				return nil, resp.err
			}
			for key, value := range resp.result.Result {
				merged[key] = value
			}
		}
	}

	if req.Type == OpRange {
		return merged, nil
	}
	return nil, nil
}

func (s *Skeen) ReceiveProtocol(ctx context.Context, msg ProtocolMessage) (ProtocolResponse, error) {
	return s.handleProtocol(ctx, msg, true)
}

func (s *Skeen) HandleProtocol(ctx context.Context, msg ProtocolMessage) (ProtocolResponse, error) {
	return s.handleProtocol(ctx, msg, false)
}

func (s *Skeen) handleProtocol(ctx context.Context, msg ProtocolMessage, waitForStart bool) (ProtocolResponse, error) {
	outbound, deliveries, waitState, err := s.applyProtocolMessage(msg)
	if err != nil {
		return ProtocolResponse{}, err
	}

	for _, delivery := range deliveries {
		s.executeDelivery(delivery)
	}

	for _, out := range outbound {
		if _, err := s.sendProtocol(ctx, out.to, out.msg); err != nil {
			return ProtocolResponse{}, err
		}
	}

	if waitForStart && msg.Type == MessageStart {
		if waitState == nil {
			return ProtocolResponse{}, fmt.Errorf("missing request state for %s", msg.RequestID)
		}
		select {
		case <-ctx.Done():
			return ProtocolResponse{}, ctx.Err()
		case <-waitState.done:
			s.mu.Lock()
			defer s.mu.Unlock()
			return ProtocolResponse{Result: cloneMap(waitState.result)}, nil
		}
	}

	return ProtocolResponse{}, nil
}

type outboundMessage struct {
	to  PartitionID
	msg ProtocolMessage
}

type pendingDelivery struct {
	requestID string
	req       Request
}

func (s *Skeen) applyProtocolMessage(msg ProtocolMessage) ([]outboundMessage, []pendingDelivery, *requestState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.ensureStateLocked(msg)
	var waitState *requestState
	if msg.Type == MessageStart {
		waitState = state
	}

	var outbound []outboundMessage
	var deliveries []pendingDelivery
	switch msg.Type {
	case MessageStart:
		if !containsPartition(msg.Request.Dst, s.id) {
			return nil, nil, nil, fmt.Errorf("partition %d is not in destination set", s.id)
		}
		if state.local == nil {
			s.clock++
			ts := Timestamp{Clock: s.clock, Partition: s.id}
			state.local = &ts
			state.proposals[s.id] = ts
			outbound = append(outbound, s.broadcastLocked(msg.Request, MessageLocalTS, ts)...)
		}
		outbound = append(outbound, s.decideFinalIfReadyLocked(state)...)

	case MessageLocalTS:
		if !containsPartition(state.req.Dst, msg.From) {
			return nil, nil, nil, fmt.Errorf("proposal from non-destination partition %d", msg.From)
		}
		state.proposals[msg.From] = msg.Timestamp
		outbound = append(outbound, s.decideFinalIfReadyLocked(state)...)

	case MessageFinalTS:
		if !containsPartition(state.req.Dst, msg.From) {
			return nil, nil, nil, fmt.Errorf("final timestamp from non-destination partition %d", msg.From)
		}
		if err := s.learnFinalLocked(state, msg.Timestamp); err != nil {
			return nil, nil, nil, err
		}
		outbound = append(outbound, s.ackIfNeededLocked(state)...)

	case MessageACK:
		if !containsPartition(state.req.Dst, msg.From) {
			return nil, nil, nil, fmt.Errorf("ack from non-destination partition %d", msg.From)
		}
		state.acks[msg.From] = true

	default:
		return nil, nil, nil, fmt.Errorf("unknown protocol message type: %s", msg.Type)
	}

	deliveries = append(deliveries, s.tryDeliverLocked()...)
	return outbound, deliveries, waitState, nil
}

func (s *Skeen) ensureStateLocked(msg ProtocolMessage) *requestState {
	if state, ok := s.states[msg.RequestID]; ok {
		if state.req.ID == "" && msg.Request.ID != "" {
			state.req = msg.Request
		}
		return state
	}

	req := msg.Request
	if req.ID == "" {
		req.ID = msg.RequestID
	}
	state := &requestState{
		req:       req,
		proposals: make(map[PartitionID]Timestamp),
		acks:      make(map[PartitionID]bool),
		done:      make(chan struct{}),
	}
	s.states[msg.RequestID] = state
	return state
}

func (s *Skeen) decideFinalIfReadyLocked(state *requestState) []outboundMessage {
	if state.final != nil || len(state.req.Dst) == 0 || len(state.proposals) < len(state.req.Dst) {
		return nil
	}
	for _, dst := range state.req.Dst {
		if _, ok := state.proposals[dst]; !ok {
			return nil
		}
	}

	var final Timestamp
	for i, dst := range state.req.Dst {
		ts := state.proposals[dst]
		if i == 0 || final.Less(ts) {
			final = ts
		}
	}
	if err := s.learnFinalLocked(state, final); err != nil {
		return nil
	}

	outbound := s.broadcastLocked(state.req, MessageFinalTS, final)
	outbound = append(outbound, s.ackIfNeededLocked(state)...)
	return outbound
}

func (s *Skeen) learnFinalLocked(state *requestState, ts Timestamp) error {
	if state.final != nil {
		if !state.final.Equal(ts) {
			return fmt.Errorf("conflicting final timestamp for %s: got %+v existing %+v", state.req.ID, ts, *state.final)
		}
		return nil
	}
	state.final = &ts
	if s.clock < ts.Clock {
		s.clock = ts.Clock
	}
	return nil
}

func (s *Skeen) ackIfNeededLocked(state *requestState) []outboundMessage {
	if s.mode != ModeStrengthened || state.final == nil || state.ackSent {
		return nil
	}
	state.ackSent = true
	state.acks[s.id] = true

	outbound := make([]outboundMessage, 0, len(state.req.Dst)-1)
	for _, dst := range state.req.Dst {
		if dst == s.id {
			continue
		}
		outbound = append(outbound, outboundMessage{
			to: dst,
			msg: ProtocolMessage{
				Type:      MessageACK,
				From:      s.id,
				RequestID: state.req.ID,
				Request:   state.req,
				Timestamp: *state.final,
			},
		})
	}
	return outbound
}

func (s *Skeen) broadcastLocked(req Request, typ ProtocolMessageType, ts Timestamp) []outboundMessage {
	outbound := make([]outboundMessage, 0, len(req.Dst))
	for _, dst := range req.Dst {
		outbound = append(outbound, outboundMessage{
			to: dst,
			msg: ProtocolMessage{
				Type:      typ,
				From:      s.id,
				RequestID: req.ID,
				Request:   req,
				Timestamp: ts,
			},
		})
	}
	return outbound
}

func (s *Skeen) tryDeliverLocked() []pendingDelivery {
	var deliveries []pendingDelivery

	for {
		candidates := s.finalUndeliveredLocked()
		if len(candidates) == 0 {
			return deliveries
		}

		deliveredOne := false
		for _, state := range candidates {
			if !s.canDeliverLocked(state) {
				continue
			}
			state.delivered = true
			deliveries = append(deliveries, pendingDelivery{requestID: state.req.ID, req: state.req})
			deliveredOne = true
			break
		}
		if !deliveredOne {
			return deliveries
		}
	}
}

func (s *Skeen) finalUndeliveredLocked() []*requestState {
	var states []*requestState
	for _, state := range s.states {
		if state.final != nil && !state.delivered {
			states = append(states, state)
		}
	}
	sort.Slice(states, func(i, j int) bool {
		return states[i].final.Less(*states[j].final)
	})
	return states
}

func (s *Skeen) canDeliverLocked(candidate *requestState) bool {
	if candidate.final == nil || candidate.delivered {
		return false
	}
	if s.mode == ModeStrengthened {
		for _, dst := range candidate.req.Dst {
			if !candidate.acks[dst] {
				return false
			}
		}
	}

	for _, other := range s.states {
		if other == candidate || other.delivered || other.local == nil {
			continue
		}
		if other.final != nil {
			if !candidate.final.Less(*other.final) {
				return false
			}
			continue
		}
		if !candidate.final.Less(*other.local) {
			return false
		}
	}
	return true
}

func (s *Skeen) executeDelivery(delivery pendingDelivery) {
	log.Printf("[P%d] DELIVER id=%s type=%s", s.id, delivery.req.ID, delivery.req.Type)
	var result map[int]int

	switch delivery.req.Type {
	case OpPut:
		if delivery.req.Put != nil {
			s.store.Put(delivery.req.Put.Key, delivery.req.Put.Value)
		}
	case OpRange:
		if delivery.req.Range != nil {
			result = s.store.Range(delivery.req.Range.Start, delivery.req.Range.End)
		}
	}

	s.mu.Lock()
	state := s.states[delivery.requestID]
	if state == nil {
		s.mu.Unlock()
		return
	}
	state.result = result
	done := state.done
	hook := s.onDeliver
	s.mu.Unlock()

	if hook != nil {
		hook(s.id, delivery.req)
	}
	close(done)
}

func (s *Skeen) sendProtocol(ctx context.Context, to PartitionID, msg ProtocolMessage) (ProtocolResponse, error) {
	s.mu.Lock()
	transport := s.transport
	s.mu.Unlock()
	if transport == nil {
		return ProtocolResponse{}, fmt.Errorf("missing transport")
	}
	return transport.Send(ctx, to, msg)
}

func containsPartition(partitions []PartitionID, id PartitionID) bool {
	for _, partition := range partitions {
		if partition == id {
			return true
		}
	}
	return false
}

func cloneMap(in map[int]int) map[int]int {
	if in == nil {
		return nil
	}
	out := make(map[int]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
