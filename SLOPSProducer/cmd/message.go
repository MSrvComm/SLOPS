package main

import (
	"hash/fnv"
	"log"
	"math/rand"
	"time"

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

	// Count key size.
	if rand.Float64() >= app.conf.SampleThreshold {
		app.ch <- input.Key // Send the key to the lossy counter.
	}
	log.Println("message sending")
	// Use the basic version.
	if app.vanilla {
		log.Println("Basic Kafka")
		partition, err := hash(input.Key, app.conf.Partitions)
		if err != nil {
			log.Printf("Failed to hash key %s with error %v\n", input.Key, err)
		}
		go app.Produce(input.Key, input.Body, partition)
	} else { // Use the SLOPS algorithm.
		log.Println("SMALOPS")
		rand.Seed(time.Now().UnixNano())
		// if rand.Float64() >= app.conf.SampleThreshold {
		// 	app.ch <- input.Key // Send the key to the lossy counter.
		// }
		var partition int32
		if rec, err := app.keyMap.GetKey(input.Key); err != nil { // Use KeyMap to decide partition.
			// partition = app.randomPartitioner.Partition(app.conf.Partitions)
			partition, err = hash(input.Key, app.conf.Partitions)
			if err != nil {
				log.Printf("Failed to hash key %s with error %v\n", input.Key, err)
			}
		} else {
			partition = rec.Partition
			// Message Set header will be added by `Producer` when message is sent.
			if rec.Delete {
				app.backupKeyMap.Del(rec.Key) // Delete in the backup key map only.
			}
		}
		go app.Produce(input.Key, input.Body, partition)
		log.Println("Keymap size:", app.keyMap.Len())
	}

	log.Println("Received new request:", input)
}

func hash(key string, numPartitions int32) (int32, error) {
	hasher := fnv.New32a()
	hasher.Reset()
	_, err := hasher.Write([]byte(key))
	if err != nil {
		return -1, err
	}
	partition := (int32(hasher.Sum32()) & 0x7fffffff) % numPartitions
	return partition, nil
}

// func randomPartition(numPartitions int32) int32 {
// 	generator := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
// 	return int32(generator.Intn(int(numPartitions)))
// }
