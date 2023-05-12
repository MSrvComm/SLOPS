package internal

import (
	"errors"
	"math"
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

// Add or update a hot key to partition map.
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

// Get the partition a hot key is mapped to.
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

// Delete a hot key from the mappings.
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

// Avg weight on the partition.
// weight = size of key (frequence of incoming messages)
func (p *PartitionMap) partitionWtAvg(partition int32) (float64, error) {
	pTotalWt := 0

	for _, kc := range p.KV[partition] {
		pTotalWt += kc.Count
	}
	if len(p.KV[partition]) > 0 {
		return float64(pTotalWt) / float64(len(p.KV[partition])), nil
	}
	return 0, errors.New("no key on partition")
}

// Avg weight across the gateway.
func (p *PartitionMap) systemWtAvg() (float64, error) {
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
		return 0, errors.New("no hot key on the system")
	}
	return float64(sysTotalWt) / float64(len(partitionWts)), nil
}

// Get the difference in the partition weight and the system weight
func (p *PartitionMap) getWtDiff(partition int32) (float64, error) {
	pWtAvg, err := p.partitionWtAvg(partition)
	if err != nil {
		return 0, err
	}
	sysWtAvg, err := p.systemWtAvg()
	if err != nil {
		return 0, err
	}
	return pWtAvg - sysWtAvg, nil
}

// Key set from an heavier than avg partition that needs to be moved.
func (p *PartitionMap) keySet2Move(partition int32) ([]struct {
	key string
	wt  int
}, error) {
	wtDiff, err := p.getWtDiff(partition)
	if err != nil {
		return []struct {
			key string
			wt  int
		}{}, err
	}

	var keySet []struct {
		key string
		wt  int
	}

	for _, kc := range p.KV[partition] {
		if float64(kc.Count) <= wtDiff {
			keySet = append(keySet, struct {
				key string
				wt  int
			}{kc.Key, kc.Count})
		}
	}
	return keySet, nil
}

// Get partitions that are below system avg.
// And the ones that are above the system avg.
func (p *PartitionMap) getPartitionSets() ([]int32, []int32, error) {
	sysWtAvg, err := p.systemWtAvg()
	if err != nil {
		return nil, nil, err
	}

	var lessThanAvgWtPartitions []int32
	var grtrThanAvgWtPartitions []int32

	for partition := range p.KV {
		pWtAvg, err := p.partitionWtAvg(partition)
		if err != nil {
			lessThanAvgWtPartitions = append(lessThanAvgWtPartitions, partition)
			continue
		}

		if sysWtAvg > pWtAvg {
			lessThanAvgWtPartitions = append(lessThanAvgWtPartitions, partition)
		} else if sysWtAvg < pWtAvg {
			grtrThanAvgWtPartitions = append(grtrThanAvgWtPartitions, partition)
		}
	}
	return lessThanAvgWtPartitions, grtrThanAvgWtPartitions, nil
}

// Check which partition a hot key with weight "count" would best fit into.
// The target partition value is -1, if no fit is found.
func (p *PartitionMap) checkBestFit(count, partition int32, lessThanAvgWtPartitions []int32) int32 {
	targetPartition := int32(-1)
	var delta float64
	// Pick a target partition, which has the smallest delta.
	for _, prtn := range lessThanAvgWtPartitions {
		wtDiff, err := p.getWtDiff(partition)
		if err != nil {
			continue
		}
		// Find a target partition with the smallest deviation from the system avg.
		// If targetPartition == - 1, then we have not set any target yet.
		if targetPartition == -1 || float64(count) < wtDiff {
			if math.Abs(wtDiff-float64(count)) < delta {
				delta = math.Abs(wtDiff - float64(count))
				targetPartition = prtn
			}
		}
	}

	return targetPartition
}

func (p *PartitionMap) Rebalance() ([]struct {
	Key             string
	SourcePartition int32
	TargetPartition int32
	Count           int
}, error) {
	var keysMoved []struct {
		Key             string
		SourcePartition int32
		TargetPartition int32
		Count           int
	}
	// 1. Get the partition sets - getPartitionSets.
	lessThanAvgWtPartitions, grtrThanAvgWtPartitions, err := p.getPartitionSets()
	if err != nil {
		return []struct {
			Key             string
			SourcePartition int32
			TargetPartition int32
			Count           int
		}{}, err
	}
	// 2. Walk through the greater than partitions.
	for _, partition := range grtrThanAvgWtPartitions {
		// 3. For each partition in grtrThanAvgWtPartitions
		//    create keySet2Move.
		keySet, err := p.keySet2Move(partition)
		if err != nil {
			continue
		}

		// 4. For each key in keySet2Move
		for _, key := range keySet {
			// Check if it can be accomodated in one of the
			// lesser than partitions -> checkBestFit
			targetPartition := p.checkBestFit(int32(key.wt), partition, lessThanAvgWtPartitions)
			// As we move hot keys away from an overloaded partition,
			// it may eventually become the best target partition for the next keys.
			if targetPartition == partition {
				// Not overloaded anymore
				// Break out of the keySet loop.
				// And look at the next partition.
				break
			}
			// No good partition was found.
			if targetPartition == -1 {
				continue
			}

			// 5. Remove key from current partition.
			p.DelHotKey(key.key)
			// 6. Add to target partition.
			p.AddHotKey(targetPartition, key.key, key.wt)
			// 7. Add to changes to be returned.
			keysMoved = append(keysMoved, struct {
				Key             string
				SourcePartition int32
				TargetPartition int32
				Count           int
			}{key.key, partition, targetPartition, key.wt})
		}
	}
	return keysMoved, nil
}
