package main

import (
	"log"
	"math"
	"sync"
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
				}
			}
			currentBucket++
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
