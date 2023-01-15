package main

import (
	"log"

	"github.com/gin-gonic/gin"
)

func (app *Application) NewMessage(c *gin.Context) {
	var input struct {
		Key  string `json:"key"`
		Body string `json:"body"`
	}

	err := app.readJSON(c, &input)
	if err != nil {
		app.badRequestResponse(c, err)
		return
	}

	log.Println("message sending")
	// Use the basic version.
	if app.vanilla {
		go app.Produce(input.Key, input.Body)
	} else { // Use the SLOPS algorithm.
		app.ch <- input.Key // Send the key to the lossy counter.
		var partition int32
		if rec, err := app.keyMap.GetKey(input.Key); err != nil {
			partition = app.randomPartitioner.Partition(app.conf.Partitions)
		} else {
			partition = rec.Partition
			// Message Set header will be added by `Producer` when message is sent.
			if rec.Delete {
				app.keyMap.Del(rec.Key)
			}
		}
		go app.Produce(input.Key, input.Body, partition)
		log.Println("Keymap size:", app.keyMap.Len())
	}

	log.Println("Received new request:", input)
}

// func hash(key string, numPartitions int32) (int32, error) {
// 	hasher := fnv.New32a()
// 	hasher.Reset()
// 	_, err := hasher.Write([]byte(key))
// 	if err != nil {
// 		return -1, err
// 	}
// 	partition := (int32(hasher.Sum32()) & 0x7fffffff) % numPartitions
// 	return partition, nil
// }

// func randomPartition(numPartitions int32) int32 {
// 	generator := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
// 	return int32(generator.Intn(int(numPartitions)))
// }
