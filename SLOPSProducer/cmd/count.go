package main

import (
	"log"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/MSrvComm/SLOPSProducer/internal"
)

func (app *Application) ExactCount(wg *sync.WaitGroup) {
	defer wg.Done()

	// Hold the hot key records.
	items := make([]Record, 0)

	ticker := time.NewTicker(30 * time.Second)

	for {
		select {
		case <-ticker.C:
			for _, rec := range items {
				// Calculate the weight of keys that are not being deleted.
				weight := float64(rec.Count) / float64(app.conf.Threshold)
				// Check if it is already mapped to a partition.
				if keyrec, err := app.keyMap.GetKey(rec.Key); err != nil {
					p := app.MapToPartition(rec) // Get a mapping to a partition.
					// Lock the partition weights and update.
					app.Lock()
					app.partitionWeights[p] += weight
					app.Unlock()
					// Add the new mapping.
					app.keyMap.AddKey(internal.KeyRecord{Key: rec.Key, Count: rec.Count, Partition: p})
				} else { // Mapping already exists.
					// Check if weight change is over threshold.
					oldWeight := float64(keyrec.Count) / float64(app.conf.Threshold)
					// If the change in weight is substantial, re-map the partition.
					if math.Abs(oldWeight-weight)/oldWeight < float64(app.conf.ChgPercent/100.0) {
						// Adjust the weight compared to old weight.
						weight -= oldWeight
						// Update the weight on the partition.
						app.Lock()
						app.partitionWeights[keyrec.Partition] += weight
						app.Unlock()
					} else {
						// Remove the flow from the old partition.
						app.Lock()
						app.partitionWeights[keyrec.Partition] -= oldWeight
						app.Unlock()
						// Map it to a new partition.
						p := app.MapToPartition(rec)
						app.Lock()
						app.partitionWeights[p] += weight
						app.Unlock()
						// Add the new mapping.
						app.keyMap.AddKey(internal.KeyRecord{Key: rec.Key, Count: rec.Count, Partition: p})
					}
				}
			}
			log.Println(items)
		case key := <-app.ch:
			// Key already is known.
			if index, b := checkKeyList(key, &items); b {
				items[index].Count++
			} else { // Adding new key.
				rec := Record{Key: key, Count: 1}
				items = append(items, rec)
			}
		}
	}
}

func (app *Application) LossyCount(wg *sync.WaitGroup) {
	defer wg.Done()

	// Hold the hot key records.
	items := make([]Record, 0)
	currentBucket := 1
	N := 0
	epsilon := 0.1 // The error we are willing to withstand.

	// Reset stream length every 30 seconds.
	ticker := time.NewTicker(30 * time.Second)

	for {
		select {
		case <-ticker.C:
			N = 0
		case key := <-app.ch:
			N++

			// Key already is known.
			if index, b := checkKeyList(key, &items); b {
				items[index].Count++
			} else { // Adding new key.
				rec := Record{Key: key, Count: 1, Bucket: currentBucket - 1}
				items = append(items, rec)
			}

			// // Self Adjusting Width
			width := int(math.Floor(float64(N) / epsilon))
			if width < int(math.Floor(1/epsilon)) {
				width = int(math.Floor(1 / epsilon))
			}

			// Once the bucket turns over.
			if N%width == 0 {
				newItems := make([]Record, 0)
				for _, rec := range items {
					if rec.Count+rec.Bucket >= currentBucket {
						newItems = append(newItems, rec)
						// Calculate the weight of keys that are not being deleted.
						weight := float64(rec.Count) / float64(app.conf.Threshold)
						// Check if it is already mapped to a partition.
						if keyrec, err := app.keyMap.GetKey(rec.Key); err != nil {
							p := app.MapToPartition(rec) // Get a mapping to a partition.
							// Lock the partition weights and update.
							app.Lock()
							app.partitionWeights[p] += weight
							app.Unlock()
							// Add the new mapping.
							app.keyMap.AddKey(internal.KeyRecord{Key: rec.Key, Count: rec.Count, Partition: p})
						} else { // Mapping already exists.
							// Check if weight change is over threshold.
							oldWeight := float64(keyrec.Count) / float64(app.conf.Threshold)
							// If the change in weight is substantial, re-map the partition.
							if math.Abs(oldWeight-weight)/oldWeight < float64(app.conf.ChgPercent/100.0) {
								// Adjust the weight compared to old weight.
								weight -= oldWeight
								// Update the weight on the partition.
								app.Lock()
								app.partitionWeights[keyrec.Partition] += weight
								app.Unlock()
							} else {
								// Remove the flow from the old partition.
								app.Lock()
								app.partitionWeights[keyrec.Partition] -= oldWeight
								app.Unlock()
								// Map it to a new partition.
								p := app.MapToPartition(rec)
								app.Lock()
								app.partitionWeights[p] += weight
								app.Unlock()
								// Add the new mapping.
								app.keyMap.AddKey(internal.KeyRecord{Key: rec.Key, Count: rec.Count, Partition: p})
							}
						}
					}
				}
				// Increment bucket.
				currentBucket++
				// Switch items.
				items = newItems
			}

			log.Println("N:", N)
			log.Println("Current Bucket:", currentBucket)
			log.Println(items)
		}
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

func (app *Application) MapToPartition(rec Record) int {
	if app.p2c {
		rand.Seed(time.Now().UnixNano())
		p1 := rand.Intn(app.conf.Partitions)
		p2 := rand.Intn(app.conf.Partitions)

		v1 := app.partitionWeights[p1]
		v2 := app.partitionWeights[p2]

		if v1 > v2 {
			return p2
		}
		return p1
	} else {
		part := 0
		for p := 1; p < app.conf.Partitions; p++ {
			if app.partitionWeights[part] > app.partitionWeights[p] {
				part = p
			}
		}
		return part
	}
}
