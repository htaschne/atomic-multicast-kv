package main

import (
	"fmt"
	"log"
	"slices"
	"sync/atomic"
)

var (
	requestCounter uint64
)

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
	ID   string
	Type OpType
	Dst  []PartitionID

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

type Skeen struct {
	id PartitionID
}

func NewSkeen(id PartitionID) *Skeen {
	return &Skeen{id: id}
}

func (s *Skeen) deliver(req Request) (any, error) {
	log.Printf("[P%d] DELIVER id=%s type=%s", s.id, req.ID, req.Type)
	switch req.Type {
	case OpPut:
		put(req.Put.Key, req.Put.Value)
		return nil, nil

	case OpRange:
		return getRange(req.Range.Start, req.Range.End), nil

	default:
		return nil, fmt.Errorf("unknown op type: %s", req.Type)
	}
}

func (s *Skeen) Submit(req Request) (any, error) {
	log.Printf("[P%d] SUBMIT id=%s type=%s dst=%v", s.id, req.ID, req.Type, req.Dst)
	if err := req.Validate(); err != nil {
		log.Printf("[P%d] INVALID id=%s err=%v", s.id, req.ID, err)
		return nil, err
	}

	if !slices.Contains(req.Dst, s.id) {
		log.Printf("[P%d] REJECT id=%s not in dst=%v", s.id, req.ID, req.Dst)
		return nil, fmt.Errorf("partition %d is not in destination set", s.id)
	}

	return s.deliver(req)
}
