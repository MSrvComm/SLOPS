package internal

import (
	"log"
	"math"
	"os"
	"strconv"
	"sync"
	"time"
)

// KeyRecord stores the metadata for a flow.
type KeyRecord struct {
	Key       string    // The key identifying a flow.
	Count     uint64    // The `size` of the flow.
	Partition int       // The partition this key is mapped to.
	timer     time.Time // The last time the flow identified by this key was migrated.
}

// PartitionMap stores the flows that have been mapped to each partition.
type PartitionMap struct {
	storeMu                sync.RWMutex          // Lock the struct before making changes to the store.
	sleepTime              time.Duration         // Avoid Rebalance() busy loop.
	loadImbalanceTolerance int                   // Percentage in load imbalance to be tolerated.
	migrationInterval      time.Duration         // No flow will be migrated twice during this interval.
	store                  map[int][]*KeyRecord  // A store of flows mapped to partitions.
	keyMap                 map[string]*KeyRecord // Points to the key record of each key.
}

// Return a new Partition Map
func NewPartitionMap(migrationInterval int) *PartitionMap {
	wt_tol, err := strconv.ParseInt(os.Getenv("LOAD_IMBALANCE_TOLERANCE"), 10, 0)
	if err != nil {
		log.Fatal(err)
	}

	sleepTime, err := strconv.ParseInt(os.Getenv("REBALANCE_SLEEP_TIME"), 10, 0)
	if err != nil {
		log.Fatal(err)
	}

	return &PartitionMap{
		loadImbalanceTolerance: int(wt_tol),
		sleepTime:              time.Duration(sleepTime),
		migrationInterval:      time.Duration(migrationInterval),
		store:                  map[int][]*KeyRecord{},
		keyMap:                 map[string]*KeyRecord{},
	}
}

// PopulateMaps initializes the stores given the number of partitions.
func (pm *PartitionMap) PopulateMaps(partitions int) {
	for p := 0; p < partitions; p++ {
		pm.store[p] = make([]*KeyRecord, 0)
	}
}

// addKey adds a key to the store.
// It is only called from the Rebalance function and thus does not use locking.
// Rebalance already takes the locks.
func (pm *PartitionMap) addKey(key string, count uint64, partition int) {
	kc := KeyRecord{Key: key, Count: count, Partition: partition, timer: time.Now()}
	pm.store[partition] = append(pm.store[partition], &kc)
	pm.keyMap[key] = &kc
}

// AddKey adds a key to the store.
func (pm *PartitionMap) AddKey(key string, count uint64, partition int) {
	pm.storeMu.Lock()
	defer pm.storeMu.Unlock()

	pm.addKey(key, count, partition)
}

// getKey searches and returns the key metadata from the store.
// Return nil if key not found.
func (pm *PartitionMap) getKey(key string) *KeyRecord {
	kc, ok := pm.keyMap[key]
	if ok {
		return kc
	}

	return nil
}

// GetKey searches and returns the key metadata from the store.
// Return nil if key not found.
func (pm *PartitionMap) GetKey(key string) *KeyRecord {
	pm.storeMu.RLock()
	defer pm.storeMu.RUnlock()

	return pm.getKey(key)
}

// deleteKey deletes key from partition in the store.
// Return key metadata or nil if not found.
func (pm *PartitionMap) deleteKey(key string) *KeyRecord {
	for _, kcArr := range pm.store {
		for i, kc := range kcArr {
			if kc.Key == key {
				pm.store[kc.Partition] = append(pm.store[kc.Partition][:i], pm.store[kc.Partition][i+1:]...) // Delete from the store.
				delete(pm.keyMap, key)                                                                       // Delete from the keymap.
				return kc
			}
		}
	}

	return nil
}

// DeleteKey deletes key from partition.
// Returns key metadata or nil if not found.
func (pm *PartitionMap) DeleteKey(key string) *KeyRecord {
	pm.storeMu.Lock()
	defer pm.storeMu.Unlock()

	return pm.deleteKey(key)
}

// MigrateKey moves a key from one partition to another.
func (pm *PartitionMap) migrateKey(key string, count uint64, dstPartition int) {
	pm.storeMu.Lock()
	defer pm.storeMu.Unlock()
	// If key already exists.
	kc := pm.getKey(key)
	if kc != nil {
		// Remove from old partition.
		pm.deleteKey(key)
	}
	// Add to new partition.
	pm.addKey(key, count, dstPartition)
}

func (pm *PartitionMap) systemAvgSize() float64 {

	total := 0.0
	for _, kcArr := range pm.store {
		for _, kc := range kcArr {
			total += float64(kc.Count)
		}
	}

	return total / float64(len(pm.store))
}

// SystemAvgSize calculates and returns the current average size of the proxy across partitions.
func (pm *PartitionMap) SystemAvgSize() float64 {
	pm.storeMu.RLock()
	defer pm.storeMu.RUnlock()

	return pm.systemAvgSize()
}

func (pm *PartitionMap) partitionSize(partition int) float64 {
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

// PartitionSize calculates and returns the total size of a partition.
func (pm *PartitionMap) PartitionSize(partition int) float64 {
	pm.storeMu.RLock()
	defer pm.storeMu.RUnlock()

	return pm.partitionSize(partition)
}

// Check if the load on the partitions differ by more than the tolerance level.
func (pm *PartitionMap) checkTolerance() bool {
	pm.storeMu.RLock()
	partitionSizes := make([]float64, len(pm.store))
	for p := range pm.store {
		partitionSizes = append(partitionSizes, pm.partitionSize(p))
	}
	pm.storeMu.RUnlock()

	maxLoad, minLoad := 0.0, math.Inf(1)
	for _, load := range partitionSizes {
		if load > maxLoad {
			maxLoad = load
		} else if load < minLoad {
			minLoad = load
		}
	}

	return (maxLoad-minLoad)/minLoad >= (float64(pm.loadImbalanceTolerance)/100)*minLoad
}

// Rebalance the store.
func (pm *PartitionMap) Rebalance() {
	for {
		time.Sleep(pm.sleepTime * time.Microsecond)
		if !pm.checkTolerance() {
			continue
		}
		// Divide partitions into greater-than and lesser-than sets.
		lessThanParts, grtrThanParts := pm.partitionSets()
		// For each partition in grtrThanParts
		for _, p := range *grtrThanParts {
			// Select the set to be migrated
			candidates := pm.migrationCandidates(p)
			if candidates == nil || len(*candidates) == 0 {
				continue
			}
			// Find target partitions for each candidate flow.
			swapMap := pm.targetMatch(candidates, lessThanParts)
			for p, kcArr := range *swapMap {
				for _, kc := range kcArr {
					pm.migrateKey(kc.Key, kc.Count, p)
				}
			}
		}
	}
}

// partitionSets returns the current grtrThanParts and lessThanParts.
func (pm *PartitionMap) partitionSets() (*[]int, *[]int) {
	pm.storeMu.RLock()
	defer pm.storeMu.RUnlock()

	sysAvg := pm.systemAvgSize()
	lessThanParts := make([]int, 0)
	grtrThanParts := make([]int, 0)
	for p := 0; p < len(pm.store); p++ {
		pSize := pm.partitionSize(p)
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
	pm.storeMu.RLock()
	defer pm.storeMu.RUnlock()

	diff := pm.partitionSize(partition) - pm.systemAvgSize()
	if diff <= 0 {
		return nil
	}
	candidates := make([]KeyRecord, 0)
	for p, kcArr := range pm.store {
		if p == partition {
			for _, kc := range kcArr {
				if time.Since(kc.timer) > time.Duration(pm.migrationInterval) {
					candidates = append(candidates, *kc)
				}
			}
		}
	}

	return &candidates
}

// targetMatch will find the best match for each key in candidates and return a pointer to the mappings.
func (pm *PartitionMap) targetMatch(candidates *[]KeyRecord, lessThanParts *[]int) *map[int][]KeyRecord {
	pm.storeMu.RLock()
	defer pm.storeMu.RUnlock()

	swapMap := make(map[int][]KeyRecord)
	srcSize := pm.partitionSize((*candidates)[0].Partition)
	sysAvg := pm.systemAvgSize()
	for _, kc := range *candidates {
		dstPartition := kc.Partition
		dstDiff := math.Inf(1) // Positive infinity.
		for _, partition := range *lessThanParts {
			dstSize := pm.partitionSize(partition)
			// Stopping condition.
			if srcSize < dstSize || srcSize-dstSize < math.Abs(srcSize-dstSize+2*float64(kc.Count)) {
				continue
			}
			// Best Match.
			delta := math.Abs(dstSize + float64(kc.Count) - sysAvg)
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
