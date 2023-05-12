package main

import (
	"log"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/MSrvComm/SLOPSProducer/internal"
)

func (app *Application) LossyCount(wg *sync.WaitGroup) {
	defer wg.Done()

	// Hold the hot key records
	items := make([]Record, 0)
	currentBucket := 1
	N := 0
	width := int(math.Floor(1 / app.conf.Epsilon))

	for {
		key := <-app.ch
		N++

		// Key is already known
		if index, b := checkKeyList(key, &items); b {
			items[index].Count++
		} else {
			// Adding new key
			rec := Record{Key: key, Count: 1, Bucket: currentBucket - 1}
			items = append(items, rec)
		}

		// The bucket turns over.
		if N >= width {
			// Do not change items while iterating over it.
			newItems := make([]Record, 0)
			for index, rec := range items {
				// Reduce the count of each item
				items[index].Count--
				if rec.Count+rec.Bucket >= currentBucket {
					// This item stays
					newItems = append(newItems, rec)

					// If value is above a threshold.
					if float64(rec.Count) >= (app.conf.Support-app.conf.Epsilon)*float64(N) {
						// Calculate the weight of keys that are not being deleted.
						weight := float64(rec.Count) / float64(app.conf.FreqThreshold)
						// If a new hot key is detected, add it to a partition.
						if _, err := app.backupKeyMap.GetKey(rec.Key); err != nil { // Get information from backup key map.
							p := app.MapToPartition() // Get a mapping to a partition.
							// Lock the partition weights and update.
							app.Lock()
							app.partitionWeights[p] += weight
							app.Unlock()

							app.partitionMap.AddHotKey(p, rec.Key, rec.Count) // Add new key to partition map.

							// Add the new mapping.
							app.backupKeyMap.AddKey(internal.KeyRecord{Key: rec.Key, Count: rec.Count, Partition: p}) // Add new partition mapping to backupkeymap
						}
					}
				} else {
					// Mark for deletion but do not delete this here.
					// This will be deleted next time when the `message` function sends a message for this key.
					// This allows the `message` function to send an update message to both old and new partitions.
					app.backupKeyMap.MarkForDeletion(rec.Key)
					app.partitionMap.DelHotKey(rec.Key) // We can delete it from the partition mapping.
				}
			}
			// Increment bucket
			currentBucket++
			// Reset N
			N = 0
			// Swap
			items = newItems
		}
		log.Println("N:", N)
		log.Println("Current Bucket:", currentBucket)
		log.Printf("#Keys tracking: %d\n", len(items))
	}
}

// Check if key is already being tracked.
func checkKeyList(key string, items *[]Record) (int, bool) {
	for index, rec := range *items {
		if rec.Key == key {
			return index, true
		}
	}
	return -1, false
}

// Check curre

// Create Mapping to partition for a new hot key.
func (app *Application) MapToPartition() int32 {
	if app.p2c {
		rand.Seed(time.Now().UnixNano())
		p1 := rand.Int31n(app.conf.Partitions)
		p2 := rand.Int31n(app.conf.Partitions)

		v1 := app.partitionWeights[p1]
		v2 := app.partitionWeights[p2]

		if v1 > v2 {
			return p2
		}
		return p1
	} else {
		part := int32(0)
		for p := int32(1); p < app.conf.Partitions; p++ {
			if app.partitionWeights[part] > app.partitionWeights[p] {
				part = p
			}
		}
		return part
	}
}

