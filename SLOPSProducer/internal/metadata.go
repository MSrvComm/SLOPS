package internal

import (
	"math"
	"sync"
)

// KeyRecord stores the metadata for a flow.
type KeyRecord struct {
	Key       string // The key identifying a flow.
	Count     uint64 // The `size` of the flow.
	Partition int    // The partition this key is mapped to.
	Delete    bool   // Should this key be deleted.
}

// PartitionMap stores the flows that have been mapped to each partition.
type PartitionMap struct {
	mu          sync.Mutex          // Lock the struct before making changes.
	store       map[int][]KeyRecord // A store of flows mapped to partitions.
	backupStore map[int][]KeyRecord // The backup store is used to recalculate key - partition mapping.
}

// Return a new Partition Map
func NewPartitionMap() *PartitionMap {
	return &PartitionMap{
		store:       map[int][]KeyRecord{},
		backupStore: map[int][]KeyRecord{},
	}
}

// PopulateMaps initializes the stores given the number of partitions.
func (pm *PartitionMap) PopulateMaps(partitions int) {
	for p := 0; p < partitions; p++ {
		pm.store[p] = make([]KeyRecord, 0)
		pm.backupStore[p] = make([]KeyRecord, 0)
	}
}

// Main store methods

// GetKey searches and returns the key metadata from the store.
// Return nil if key not found.
func (pm *PartitionMap) GetKey(key string) *KeyRecord {
	for _, kcArr := range pm.store {
		for _, kc := range kcArr {
			if kc.Key == key {
				return &kc
			}
		}
	}
	return nil // Key not found.
}

// // AddKey adds a key to the store.
// func (pm *PartitionMap) AddKey(key string, count uint64, partition int) {
// 	pm.mu.Lock()
// 	defer pm.mu.Unlock()

// 	pm.store[partition] = append(pm.store[partition], KeyRecord{Key: key, Count: count, Partition: partition})
// }

// // DeleteKey deletes key from partition.
// // Returns key metadata or nil if not found.
// func (pm *PartitionMap) DeleteKey(key string) *KeyRecord {
// 	for _, kcArr := range pm.store {
// 		for i, kc := range kcArr {
// 			if kc.Key == key {
// 				pm.store[kc.Partition] = append(pm.store[kc.Partition][:i], pm.store[kc.Partition][i+1:]...)
// 				return &kc
// 			}
// 		}
// 	}
// 	return nil
// }

// Backup Store methods.
// AddKeyBackupStore adds a key to the store.
func (pm *PartitionMap) AddKeyBackupStore(key string, count uint64, partition int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.store[partition] = append(pm.store[partition], KeyRecord{Key: key, Count: count, Partition: partition})
}

// GetKeyBackupStore searches and returns the key metadata from the store.
// Return nil if key not found.
func (pm *PartitionMap) GetKeyBackupStore(key string) *KeyRecord {
	for _, kcArr := range pm.store {
		for _, kc := range kcArr {
			if kc.Key == key {
				return &kc
			}
		}
	}
	return nil // Key not found.
}

// DeleteKeyBackupStore deletes key from partition.
// Returns key metadata or nil if not found.
func (pm *PartitionMap) DeleteKeyBackupStore(key string) *KeyRecord {
	for _, kcArr := range pm.store {
		for i, kc := range kcArr {
			if kc.Key == key {
				pm.store[kc.Partition] = append(pm.store[kc.Partition][:i], pm.store[kc.Partition][i+1:]...)
				return &kc
			}
		}
	}
	return nil
}

// SwapStores swaps the partition map stores.
func (pm *PartitionMap) SwapStores() {
	var p int
	for p, pm.store[p] = range pm.backupStore {
	}
}

// MigrateKey moves a key from one partition to another.
func (pm *PartitionMap) migrateKeyBackupStore(key string, count uint64, dstPartition int) {
	// If key already exists.
	kc := pm.GetKeyBackupStore(key)
	if kc != nil {
		// Remove from old partition.
		pm.DeleteKeyBackupStore(key)
	}
	// Add to new partition.
	pm.AddKeyBackupStore(key, count, dstPartition)
}

// SystemAvgSize calculates and returns the current average size of the proxy across partitions.
func (pm *PartitionMap) SystemAvgSize() float64 {
	total := 0.0
	for _, kcArr := range pm.store {
		for _, kc := range kcArr {
			total += float64(kc.Count)
		}
	}
	return total / float64(len(pm.store))
}

// PartitionSize calculates and returns the total size of a partition.
func (pm *PartitionMap) PartitionSize(partition int) float64 {
	total := 0.0
	for p, kcArr := range pm.store {
		if p == partition {
			for _, kc := range kcArr {
				total += float64(kc.Count)
			}
		}
	}
	return total
}

// Rebalance rebalances the backup store.
func (pm *PartitionMap) Rebalance() {
	// Divide partitions into greater than and lesser than sets.
	lessThanParts, grtrThanParts := pm.partitionSets()
	// For each partition in grtrThanParts
	for _, p := range *grtrThanParts {
		// Select the set to be migrated
		candidates := pm.migrationCandidates(p)
		// Find target partitions for each candidate flow.
		swapMap := pm.targetMatch(candidates, lessThanParts)
		for p, kcArr := range *swapMap {
			for _, kc := range kcArr {
				pm.migrateKeyBackupStore(kc.Key, kc.Count, p)
			}
		}
	}
}

// partitionSets returns the current grtrThanParts and lessThanParts.
func (pm *PartitionMap) partitionSets() (*[]int, *[]int) {
	sysAvg := pm.SystemAvgSize()
	lessThanParts := make([]int, 0)
	grtrThanParts := make([]int, 0)
	for p := 0; p < len(pm.store); p++ {
		pSize := pm.PartitionSize(p)
		if pSize < sysAvg {
			lessThanParts = append(lessThanParts, p)
		} else if pSize > sysAvg {
			grtrThanParts = append(grtrThanParts, p)
		}
	}
	return &lessThanParts, &grtrThanParts
}

// migrationCandidates recalculates the partitionSize and return a set of possible migration candidates.
func (pm *PartitionMap) migrationCandidates(partition int) *[]KeyRecord {
	diff := pm.PartitionSize(partition) - pm.SystemAvgSize()
	if diff <= 0 {
		return nil
	}
	candidates := make([]KeyRecord, 0)
	for p, kcArr := range pm.store {
		if p == partition {
			for _, kc := range kcArr {
				if float64(kc.Count) <= diff {
					candidates = append(candidates, kc)
				}
			}
		}
	}
	return &candidates
}

// targetMatch will find the best match for each key in candidates and return a pointer to the mappings.
func (pm *PartitionMap) targetMatch(candidates *[]KeyRecord, lessThanParts *[]int) *map[int][]KeyRecord {
	swapMap := make(map[int][]KeyRecord)
	srcSize := pm.PartitionSize((*candidates)[0].Partition)
	sysAvg := pm.SystemAvgSize()
	for _, kc := range *candidates {
		dstPartition := kc.Partition
		dstDiff := math.Inf(1) // Positive infinity.
		for _, partition := range *lessThanParts {
			dstSize := pm.PartitionSize(partition)
			// Stopping condition.
			if srcSize < dstSize || srcSize-dstSize < math.Abs(srcSize-dstSize+2*float64(kc.Count)) {
				continue
			}
			// Best Match.
			delta := math.Abs(dstSize - sysAvg - float64(kc.Count))
			if delta < dstDiff {
				dstDiff = delta
				dstPartition = partition
			}
		}
		if dstPartition == kc.Partition { // No candidate for migration was found.
			continue
		}
		swapMap[dstPartition] = append(swapMap[dstPartition], kc)
	}
	return &swapMap
}
