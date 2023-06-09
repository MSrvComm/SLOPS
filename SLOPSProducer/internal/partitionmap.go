package internal

import (
	"errors"
	"math"
	"sort"
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
	KV           map[int32][]KeyCount
	RebalanceMap map[int32][]KeyCount // This map is used as a static copy during rebalance.
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
func (p *PartitionMap) GetPartitionWt(partition int32) float64 {
	pTotalWt := 0

	for _, kc := range p.RebalanceMap[partition] {
		pTotalWt += kc.Count
	}
	// if len(p.KV[partition]) > 0 {
	// 	return float64(pTotalWt) / float64(len(p.KV[partition])), nil
	// }
	// return 0, errors.New("no key on partition")
	return float64(pTotalWt)
}

// Avg weight across the gateway.
func (p *PartitionMap) SystemWtAvg() (float64, error) {
	sysTotalWt := int(0)
	var partitions int
	for partition := range p.RebalanceMap {
		partitionWt := 0
		for _, kc := range p.RebalanceMap[partition] {
			sysTotalWt += kc.Count
			partitionWt += kc.Count
		}
		partitions++
	}
	if partitions == 0 {
		return 0, errors.New("no hot key on the system")
	}
	return float64(sysTotalWt) / float64(partitions), nil
}

// Get the difference in the partition weight and the system weight
func (p *PartitionMap) getWtDiff(partition int32) (float64, error) {
	// pWtAvg, err := p.getPartitionWt(partition)
	// if err != nil {
	// 	return 0, err
	// }
	partitionTotalWt := 0
	for _, kc := range p.RebalanceMap[partition] {
		partitionTotalWt += kc.Count
	}
	sysWtAvg, err := p.SystemWtAvg()
	if err != nil {
		return 0, err
	}
	// return pWtAvg - sysWtAvg, nil
	return float64(partitionTotalWt) - sysWtAvg, nil
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

	for _, kc := range p.RebalanceMap[partition] {
		if float64(kc.Count) <= wtDiff {
			keySet = append(keySet, struct {
				key string
				wt  int
			}{kc.Key, kc.Count})
		}
	}
	// Sort the keys
	sort.Slice(keySet, func(i, j int) bool {
		return keySet[i].wt < keySet[j].wt
	})
	return keySet, nil
}

// Get partitions that are below system avg.
// And the ones that are above the system avg.
func (p *PartitionMap) getPartitionSets() ([]int32, []int32, error) {
	sysWtAvg, err := p.SystemWtAvg()
	if err != nil {
		return nil, nil, err
	}

	var lessThanAvgWtPartitions []int32
	var grtrThanAvgWtPartitions []int32

	for partition := range p.RebalanceMap {
		// pWtAvg, err := p.partitionWt(partition)
		// if err != nil {
		// 	lessThanAvgWtPartitions = append(lessThanAvgWtPartitions, partition)
		// 	continue
		// }

		partitionWt := p.GetPartitionWt(partition)

		if sysWtAvg > partitionWt {
			lessThanAvgWtPartitions = append(lessThanAvgWtPartitions, partition)
		} else if sysWtAvg < partitionWt {
			grtrThanAvgWtPartitions = append(grtrThanAvgWtPartitions, partition)
		}
	}
	return lessThanAvgWtPartitions, grtrThanAvgWtPartitions, nil
}

// TODO: When to not migrate a key?
// If for all partitions, migrating the key makes the
// partition's total size greater than the source partition's size.
func (p *PartitionMap) checkBestFit2(count, partition int32) int32 {
	targetPartition := int32(-1)
	var delta float64
	for prtn := range p.RebalanceMap {
		if prtn == partition {
			continue
		}
		targetWt := p.GetPartitionWt(prtn)
		srcWt := p.GetPartitionWt(partition)
		if srcWt < targetWt {
			continue
		}
		// target partition is not set.
		// or the difference between the the two partitions
		// is the lowest after migrating the hot key.
		curDelta := math.Abs((srcWt - float64(count)) - (targetWt + float64(count)))
		if targetPartition == -1 || curDelta < delta {
			delta = curDelta
			targetPartition = prtn
		}
	}
	return targetPartition
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
		// if targetPartition == -1 || float64(count) < wtDiff {
		if math.Abs(wtDiff+float64(count)) < delta {
			// delta = math.Abs(wtDiff - float64(count))
			delta = math.Abs(wtDiff + float64(count))
			targetPartition = prtn
		}
		// }
	}

	return targetPartition
}

func (p *PartitionMap) Rebalance() ([]struct {
	Key             string
	SourcePartition int32
	TargetPartition int32
	Count           int
	SysWt           float64 // exists to check behavior
}, error) {
	// make a static copy of the map.
	for prtn, kcArr := range p.KV {
		p.RebalanceMap[prtn] = make([]KeyCount, len(kcArr))
		copy(kcArr, p.RebalanceMap[prtn])
	}

	var keysMoved []struct {
		Key             string
		SourcePartition int32
		TargetPartition int32
		Count           int
		SysWt           float64 // exists to check behavior
	}
	// System Avg weight
	sysAvgWt, err := p.SystemWtAvg()
	if err != nil {
		return []struct {
			Key             string
			SourcePartition int32
			TargetPartition int32
			Count           int
			SysWt           float64 // exists to check behavior
		}{}, err
	}
	// 1. Get the partition sets - getPartitionSets.
	// lessThanAvgWtPartitions, grtrThanAvgWtPartitions, err := p.getPartitionSets()
	_, grtrThanAvgWtPartitions, err := p.getPartitionSets()
	if err != nil {
		return []struct {
			Key             string
			SourcePartition int32
			TargetPartition int32
			Count           int
			SysWt           float64 // exists to check behavior
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

		// exists to check behavior
		wtDiff, err := p.getWtDiff(partition)
		if err != nil {
			return []struct {
				Key             string
				SourcePartition int32
				TargetPartition int32
				Count           int
				SysWt           float64 // exists to check behavior
			}{}, err
		}

		// 4. For each key in keySet2Move
		for _, key := range keySet {
			// Check if it can be accomodated in one of the
			// lesser than partitions -> checkBestFit
			// targetPartition := p.checkBestFit(int32(key.wt), partition, lessThanAvgWtPartitions)
			targetPartition := p.checkBestFit2(int32(key.wt), partition)
			// targetPartition := p.checkBestFit2(int32(key.wt), partition)
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
				SysWt           float64 // exists to check behavior
			}{key.key, partition, targetPartition, key.wt, wtDiff})

			// Check if partition total weight is going to fall below
			// system avg weight if this key is moved.
			// That means we have already moved enough keys from this partition.
			if p.GetPartitionWt(partition) <= sysAvgWt {
				break
			}
		}
	}
	return keysMoved, nil
}
