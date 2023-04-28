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
	N := 0

	// ticker := time.NewTicker(30 * time.Second)

	for {
		key := <-app.ch
		N++

		if N > 5000 {
			// reset N 30 seconds.
			N = 0
		}

		// Key already is known.
		if index, b := checkKeyList(key, &items); b {
			items[index].Count++
		} else { // Adding new key.
			rec := Record{Key: key, Count: 1}
			items = append(items, rec)
		}
		for _, rec := range items {
			// Reduce all counts.
			go reduceCounts(&items)
			// Calculate the weight of keys that are not being deleted.
			weight := float64(rec.Count) / float64(app.conf.FreqThreshold)
			// Check if it is already mapped to a partition.
			if keyrec, err := app.keyMap.GetKey(rec.Key); err != nil {
				p := app.MapToPartition(rec) // Get a mapping to a partition.
				// Lock the partition weights and update.
				app.Lock()
				app.partitionWeights[p] += weight
				app.Unlock()
				if rec.Count > 10 {
					// Add the new mapping.
					app.keyMap.AddKey(internal.KeyRecord{Key: rec.Key, Count: rec.Count, Partition: p})
				}
			} else { // Mapping already exists.
				if rec.Count < 1 { // remove stale flows.
					app.backupKeyMap.MarkForDeletion(keyrec.Key)
					app.Lock()
					app.partitionWeights[keyrec.Partition] -= weight
					app.Unlock()
				} else {
					// Check if weight change is over threshold.
					oldWeight := float64(keyrec.Count) / float64(app.conf.FreqThreshold)
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
	}
}

func (app *Application) LossyCount(wg *sync.WaitGroup) {
	defer wg.Done()

	// Hold the hot key records.
	items := make([]Record, 0)
	currentBucket := 1
	N := 0
	// epsilon := 0.1 // The error we are willing to withstand.
	width := int(math.Floor(1 / app.conf.Epsilon))

	for {
		key := <-app.ch
		N++

		// Key already is known.
		if index, b := checkKeyList(key, &items); b {
			items[index].Count++
		} else { // Adding new key.
			rec := Record{Key: key, Count: 1, Bucket: currentBucket - 1}
			items = append(items, rec)
		}

		// Once the bucket turns over.
		// if N%width == 0 {
		if N >= width {
			newItems := make([]Record, 0)
			for index, rec := range items {
				items[index].Count-- // Reduce count of each item.
				if rec.Count+rec.Bucket >= currentBucket {
					newItems = append(newItems, rec)
					// If value is above a threshold.
					if float64(rec.Count) >= (app.conf.Support-app.conf.Epsilon)*float64(N) {
						// Calculate the weight of keys that are not being deleted.
						weight := float64(rec.Count) / float64(app.conf.FreqThreshold)
						// Check if it is already mapped to a partition.
						if keyrec, err := app.backupKeyMap.GetKey(rec.Key); err != nil { // Get information from backup key map.
							p := app.MapToPartition(rec) // Get a mapping to a partition.
							// Lock the partition weights and update.
							app.Lock()
							app.partitionWeights[p] += weight
							app.Unlock()

							// Add the new mapping.
							// app.keyMap.AddKey(internal.KeyRecord{Key: rec.Key, Count: rec.Count, Partition: p})
							app.backupKeyMap.AddKey(internal.KeyRecord{Key: rec.Key, Count: rec.Count, Partition: p}) // Add new partition mapping to backupkeymap
						} else { // Mapping already exists.
							// Check if weight change is over threshold.
							oldWeight := float64(keyrec.Count) / float64(app.conf.FreqThreshold)
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
								app.backupKeyMap.AddKey(internal.KeyRecord{Key: rec.Key, Count: rec.Count, Partition: p})
							}
						}
					}
				} else {
					// Mark for deletion but do not delete this here.
					// This will be deleted next time when the `message` function sends a message for this key.
					// This allows the `message` function to send an update message to both old and new partitions.
					app.backupKeyMap.MarkForDeletion(rec.Key)
				}
			}
			// Increment bucket.
			currentBucket++
			// Switch items.
			items = newItems
			N = 0
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

// Create Mapping to partition for a new hot key.
func (app *Application) MapToPartition(rec Record) int32 {
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

func reduceCounts(items *[]Record) {
	for i := range *items {
		(*items)[i].Count--
	}
}
