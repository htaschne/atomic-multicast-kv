package main

import "fmt"

type Router struct {
	partitionCount int
}

func NewRouter(partitionCount int) (Router, error) {
	if partitionCount <= 0 {
		return Router{}, fmt.Errorf("partition count must be positive")
	}
	return Router{partitionCount: partitionCount}, nil
}

func MustRouter(partitionCount int) Router {
	router, err := NewRouter(partitionCount)
	if err != nil {
		panic(err)
	}
	return router
}

func (r Router) PartitionCount() int {
	return r.partitionCount
}

func (r Router) PartitionForKey(k int) PartitionID {
	return PartitionID(positiveMod(k, r.partitionCount))
}

func (r Router) DestinationsForPut(k int) []PartitionID {
	return []PartitionID{r.PartitionForKey(k)}
}

func (r Router) DestinationsForRange(start, end int) []PartitionID {
	if start > end {
		return nil
	}

	seen := make([]bool, r.partitionCount)
	dst := make([]PartitionID, 0, r.partitionCount)
	for key := start; key <= end; key++ {
		partition := r.PartitionForKey(key)
		if !seen[partition] {
			seen[partition] = true
			dst = append(dst, partition)
		}
		if len(dst) == r.partitionCount {
			break
		}
	}
	return dst
}

func (r Router) AllDestinations() []PartitionID {
	dst := make([]PartitionID, 0, r.partitionCount)
	for i := 0; i < r.partitionCount; i++ {
		dst = append(dst, PartitionID(i))
	}
	return dst
}

func partitionForKey(k int) PartitionID {
	return MustRouter(DefaultPartitionCount).PartitionForKey(k)
}

func destinationsForPut(k int) []PartitionID {
	return MustRouter(DefaultPartitionCount).DestinationsForPut(k)
}

func destinationsForRange(start, end int) []PartitionID {
	return MustRouter(DefaultPartitionCount).DestinationsForRange(start, end)
}

func positiveMod(value, divisor int) int {
	result := value % divisor
	if result < 0 {
		result += divisor
	}
	return result
}
