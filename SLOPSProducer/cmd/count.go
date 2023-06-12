package main

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// Record struct.
type Record struct {
	Key    string
	Count  uint64
	Bucket int
}

func (app *Application) LossyCount(wg *sync.WaitGroup) {
	defer wg.Done()

	// Hold hot keys.
	items := make([]Record, 0)
	currentBucket := 1
	N := 0
	width := int(math.Floor(1 / app.conf.Epsilon))

	for {
		key := <-app.ch
		N++

		// Key is known.
		if index, b := checkKeyList(key, &items); b {
			items[index].Count++
		} else {
			// Adding new key to records.
			rec := Record{Key: key, Count: 1, Bucket: currentBucket - 1}
			items = append(items, rec)
		}

		// The bucket turns over.
		if N >= width {
			// Do not change the ds while iterating over it.
			newItems := make([]Record, 0)

			for _, rec := range items {
				// Reduce count of each index.
				rec.Count--
				if rec.Count+uint64(rec.Bucket) >= uint64(currentBucket) {
					// This item stays.
					newItems = append(newItems, rec)

					// If value is above a threshold.
					if float64(rec.Count) >= (app.conf.Support-app.conf.Epsilon)*float64(N) {
						// If a new hot key is detected, add it.
						if m := app.partitionMap.GetKey(rec.Key); m == nil {
							// Map to a new partition.
							p := app.MapToPartition()
							app.mu.Lock()
							app.partitionMap.AddKeyBackupStore(rec.Key, rec.Count, p)
							app.mu.Unlock()
						}
					}
				} else {
					app.partitionMap.DeleteKeyBackupStore(rec.Key)
				}
			}
			// Increment current bucket.
			currentBucket++
			// Reset N.
			N = 0
			// Update items.
			items = newItems
		}
		app.logger.Println("N:", N)
		app.logger.Println("Current Bucket:", currentBucket)
		app.logger.Printf("#Keys tracking: %d\n", len(items))
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

// Create Mapping to partition for a new hot key.
func (app *Application) MapToPartition() int {

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	p1 := int(r.Int31n(app.conf.Partitions))
	p2 := int(rand.Int31n(app.conf.Partitions))

	v1 := app.partitionMap.PartitionSize(p1)
	v2 := app.partitionMap.PartitionSize(p2)

	if v1 > v2 {
		return p2
	}
	return p1
}
