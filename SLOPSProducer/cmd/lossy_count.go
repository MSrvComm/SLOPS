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

	items := make([]Record, 0)
	currentBucket := 1
	N := 0
	epsilon := 0.1
	for {
		key := <-app.ch
		N++

		if index, b := checkKeyList(key, &items); b {
			items[index].Count++
		} else {
			rec := Record{Key: key, Count: 1, Bucket: currentBucket - 1}
			items = append(items, rec)
		}

		width := int(math.Ceil(float64(N) * epsilon))
		if width < 10 {
			width = 10
		}
		log.Println("Width:", width)

		if N%width == 0 {
			itemsToBeDeleted := make([]int, 0)
			for index, rec := range items {
				if rec.Count+rec.Bucket < currentBucket {
					itemsToBeDeleted = append(itemsToBeDeleted, index)
				} else {
					// Calculate the weight of the key.
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
						// Adjust the weight compared to old weight.
						weight -= float64(keyrec.Count) / float64(app.conf.Threshold)
						// Update the weight on the partition.
						app.Lock()
						app.partitionWeights[keyrec.Partition] += weight
						app.Unlock()

					}
				}
			}
			// Increment bucket.
			currentBucket++
			// Delete items marked for deletion.
			for _, index := range itemsToBeDeleted {
				if index > 0 {
					items = append(items[:index-1], items[index+1:]...)
				} else {
					items = items[1:]
				}
			}
		}

		log.Println("N:", N)
		log.Println("Current Bucket:", currentBucket)
		log.Println(items)
	}
}

func checkKeyList(key string, items *[]Record) (int, bool) {
	for index, rec := range *items {
		if rec.Key == key {
			return index, true
		}
	}
	return -1, false
}

func (app *Application) MapToPartition(rec Record) int {
	rand.Seed(time.Now().UnixNano())

	p1 := rand.Intn(app.conf.Partitions)
	p2 := rand.Intn(app.conf.Partitions)

	v1 := app.partitionWeights[p1]
	v2 := app.partitionWeights[p2]

	if v1 > v2 {
		return p2
	}
	return p1
}
