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
	storeMu     sync.Mutex          // Lock the struct before making changes to the store.
	bstoreMu    sync.Mutex          // Lock the struct before making changes to the backup store.
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
	pm.storeMu.Lock()
	defer pm.storeMu.Unlock()
	for _, kcArr := range pm.store {
		for _, kc := range kcArr {
			if kc.Key == key {
				return &kc
			}
		}
	}
	return nil // Key not found.
}

// Backup Store methods.

// addKeyBackupStore adds a key to the backup store.
// It is only called from the Rebalance function and thus does not use locking.
// Rebalance already takes the locks.
func (pm *PartitionMap) addKeyBackupStore(key string, count uint64, partition int) {
	pm.backupStore[partition] = append(pm.backupStore[partition], KeyRecord{Key: key, Count: count, Partition: partition})
}

// AddKeyBackupStore adds a key to the backup store.
func (pm *PartitionMap) AddKeyBackupStore(key string, count uint64, partition int) {
	pm.bstoreMu.Lock()
	defer pm.bstoreMu.Unlock()

	pm.addKeyBackupStore(key, count, partition)
}

// getKeyBackupStore searches and returns the key metadata from the store.
// Return nil if key not found.
func (pm *PartitionMap) getKeyBackupStore(key string) *KeyRecord {
	for _, kcArr := range pm.backupStore {
		for _, kc := range kcArr {
			if kc.Key == key {
				return &kc
			}
		}
	}
	return nil // Key not found.
}

// deleteKeyBackupStore deletes key from partition in the backup store.
// Return key metadata or nil if not found.
func (pm *PartitionMap) deleteKeyBackupStore(key string) *KeyRecord {
	for _, kcArr := range pm.backupStore {
		for i, kc := range kcArr {
			if kc.Key == key {
				pm.backupStore[kc.Partition] = append(pm.backupStore[kc.Partition][:i], pm.backupStore[kc.Partition][i+1:]...)
				return &kc
			}
		}
	}
	return nil
}

// DeleteKeyBackupStore deletes key from partition.
// Returns key metadata or nil if not found.
func (pm *PartitionMap) DeleteKeyBackupStore(key string) *KeyRecord {
	pm.bstoreMu.Lock()
	defer pm.bstoreMu.Unlock()

	return pm.deleteKeyBackupStore(key)
}

// SwapStores swaps the partition map stores.
func (pm *PartitionMap) SwapStores() {
	// Both store and backup store needs to be locked for this.
	pm.storeMu.Lock()
	pm.bstoreMu.Lock()
	defer pm.storeMu.Unlock()
	defer pm.bstoreMu.Unlock()

	var p int
	for p, pm.store[p] = range pm.backupStore {
	}
}

// MigrateKey moves a key from one partition to another.
func (pm *PartitionMap) migrateKeyBackupStore(key string, count uint64, dstPartition int) {
	pm.storeMu.Lock()
	defer pm.storeMu.Unlock()
	// If key already exists.
	kc := pm.getKeyBackupStore(key)
	if kc != nil {
		// Remove from old partition.
		pm.deleteKeyBackupStore(key)
	}
	// Add to new partition.
	pm.addKeyBackupStore(key, count, dstPartition)
}

// SystemAvgSize calculates and returns the current average size of the proxy across partitions.
func (pm *PartitionMap) SystemAvgSize() float64 {
	total := 0.0
	for _, kcArr := range pm.backupStore {
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

// bPartitionSize calculates and returns the total size of a partition from the backup store.
// Used for internal calculations.
func (pm *PartitionMap) bPartitionSize(partition int) float64 {
	total := 0.0
	for p, kcArr := range pm.backupStore {
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
	// Lock backup store.
	pm.bstoreMu.Lock()
	defer pm.bstoreMu.Unlock()

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
		pSize := pm.bPartitionSize(p)
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
	diff := pm.bPartitionSize(partition) - pm.SystemAvgSize()
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
	srcSize := pm.bPartitionSize((*candidates)[0].Partition)
	sysAvg := pm.SystemAvgSize()
	for _, kc := range *candidates {
		dstPartition := kc.Partition
		dstDiff := math.Inf(1) // Positive infinity.
		for _, partition := range *lessThanParts {
			dstSize := pm.bPartitionSize(partition)
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
