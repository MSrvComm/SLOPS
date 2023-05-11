package internal

import (
	"errors"
	"sync"
)

type KeyCount struct {
	Key   string
	Count int
}

type PartitionMap struct {
	mu sync.Mutex
	// Each partition is an int32 - that is the key.
	// For each partition we want to store an array.
	// Each element of the array is the hot key mapped to that partition
	// And the count associated with that hot key.
	// So an array of KeyCount structs.
	KV map[int32][]KeyCount
}

func (p *PartitionMap) AddHotKey(partition int32, key string, count int) {
	new_kc := KeyCount{Key: key, Count: count}
	p.mu.Lock()
	defer p.mu.Unlock()

	// If the key already exists then update it.
	for i := range p.KV[partition] {
		if p.KV[partition][i].Key == key {
			p.KV[partition][i].Count = count
			return
		}
	}
	// Else add a new key.
	p.KV[partition] = append(p.KV[partition], new_kc)
}

func (p *PartitionMap) GetHotKey(key string) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for partition := range p.KV {
		for _, kc := range p.KV[partition] {
			if kc.Key == key {
				return kc.Count, nil
			}
		}
	}
	return 0, errors.New("key not found")
}

func (p *PartitionMap) DelHotKey(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for partition := range p.KV {
		for i, kc := range p.KV[partition] {
			if kc.Key == key {
				p.KV[partition] = append(p.KV[partition][:i], p.KV[partition][i+1:]...)
			}
		}
	}
}

func (p *PartitionMap) rebalance() {
	sysTotalWt := int(0)
	var partitionWts []int
	for partition := range p.KV {
		partitionWt := 0
		for _, kc := range p.KV[partition] {
			sysTotalWt += kc.Count
			partitionWt += kc.Count
		}
		partitionWts = append(partitionWts, partitionWt)
	}
	if len(partitionWts) == 0 {
		return
	}
	sysAvgWt := float64(sysTotalWt) / float64(len(partitionWts))

	var lessThanAvgWtPartitions []struct {
		partition int
		wt        int
	}
	var grtrThanAvgWtPartitions []struct {
		partition int
		wt        int
	}

	for partition, wt := range partitionWts {
		if float64(wt) < sysAvgWt {
			lessThanAvgWtPartitions = append(lessThanAvgWtPartitions, struct {
				partition int
				wt        int
			}{partition, wt})
		} else if float64(wt) > sysAvgWt {
			grtrThanAvgWtPartitions = append(grtrThanAvgWtPartitions, struct {
				partition int
				wt        int
			}{partition, wt})
		}

		
	}

}
