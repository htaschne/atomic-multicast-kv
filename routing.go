package main

func partitionForKey(k int) PartitionID {
	if k%2 == 0 {
		return 0
	}
	return 1
}

func destinationsForPut(k int) []PartitionID {
	return []PartitionID{partitionForKey(k)}
}

func destinationsForRange(start, end int) []PartitionID {
	hasEven := false
	hasOdd := false

	for i := start; i <= end; i++ {
		if i%2 == 0 {
			hasEven = true
		} else {
			hasOdd = true
		}
	}

	dst := make([]PartitionID, 0, 2)
	if hasEven {
		dst = append(dst, 0)
	}
	if hasOdd {
		dst = append(dst, 1)
	}
	return dst
}
