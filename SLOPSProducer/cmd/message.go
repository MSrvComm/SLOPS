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
		rand.New(rand.NewSource(time.Now().UnixNano()))
		// rand.Seed(time.Now().UnixNano())
		var partition int32
		if rec := app.partitionMap.GetKey(input.Key); rec == nil { // Use KeyMap to decide partition.
			partition, err = hash(input.Key, app.conf.Partitions)
			if err != nil {
				log.Printf("Failed to hash key %s with error %v\n", input.Key, err)
			}
		} else {
			partition = int32(rec.Partition)
			// Message Set header will be added by `Producer` when message is sent.
		}
		go app.Produce(input.Key, input.Body, partition)
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
