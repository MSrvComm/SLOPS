package main

import (
	"log"
	"sync"
	"time"

	"github.com/MSrvComm/SLOPSProducer/internal"
)

// Application is the workhorse of the system.
type Application struct {
	sync.Mutex
	vanilla          bool                   // If true then do not use the SLOPS algorithm.
	p2c              bool                   // If true then use P2C to load balance between partitions.
	ch               chan string            // Receive incoming keys through this channel.
	conf             internal.Config        // Hold the configuration data.
	partitionMap     *internal.PartitionMap // Hot keys mapped to each partition.
	keyMap           *internal.KeyMap       // Mapping of hot keys.
	backupKeyMap     *internal.KeyMap       // Add new mappings to the backup and swap every 30 seconds.
	messageSets      *MessageSetMap         // Map Message Sets
	partitionWeights []float64              // Weights of each partition.
	logger           *log.Logger            // System level logger.
	// randomPartitioner *internal.RandomPartitioner
	producer     Producer // Kafka producer.
	mapSwapTimer time.Ticker
}

// Swap the map every interval set in the ticker.
func (app *Application) SwapMaps() {
	for range app.mapSwapTimer.C { // Swap the structs.
		// Before swapping the maps
		// rebalance the partitions.
		keysMoved, err := app.partitionMap.Rebalance()
		if err == nil || len(keysMoved) != 0 {
			// Change the key to partition mappings as well.
			for _, keyS := range keysMoved {
				weight := float64(keyS.Count) / float64(app.conf.FreqThreshold)
				// Remove the flow from the old partition.
				app.Lock()
				app.partitionWeights[keyS.SourcePartition] -= weight
				app.Unlock()
				// Map it to a new partition.
				app.Lock()
				app.partitionWeights[keyS.TargetPartition] += weight
				app.Unlock()
				// Add the new mapping.
				app.backupKeyMap.AddKey(internal.KeyRecord{Key: keyS.Key, Count: keyS.Count, Partition: keyS.TargetPartition})
			}
		}
		log.Println("Swapping the maps")
		newMap := make(map[string]internal.KeyRecord)
		for k, v := range app.backupKeyMap.KV {
			newMap[k] = v
		}
		app.keyMap.KV = newMap

		go app.debugPrint(keysMoved) // // exists to check behavior
	}
}

// exists to check behavior
func (app *Application) debugPrint(entries []struct {
	Key             string
	SourcePartition int32
	TargetPartition int32
	Count           int
	SysWt           float64 // exists to check behavior
}) {
	// var partitionAvgWtMap map[int32]float64
	// var partitionKeySetMap map[int32][]int

	for _, entry := range entries {
		// partition := entry.SourcePartition
		// partitionAvgWtMap[partition] = entry.SysWt
		// partitionKeySetMap[partition] = append(partitionKeySetMap[partition], entry.Count)
		if float64(entry.Count) > entry.SysWt {
			log.Printf("beahvior error: Partition %d with partition weight %f moved a flow with weight %d\n", entry.SourcePartition, entry.SysWt, entry.Count)
		}
	}
}
