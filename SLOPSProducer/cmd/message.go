package main

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"time"

	"github.com/gin-gonic/gin"
)

type kInput struct {
	Key  string `json:"key"`
	Body string `json:"body"`
}

func (in kInput) String() string {
	return fmt.Sprintf("Key: %s, Body: %s", in.Key, in.Body)
}

func (app *Application) NewMessage(c *gin.Context) {
	var input kInput
	err := app.readJSON(c, &input)
	if err != nil {
		app.badRequestResponse(c, err)
		return
	}

	// Count key size.
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	if r.Float64() >= app.conf.SampleThreshold {
		app.ch <- input.Key // Send the key to the lossy counter.
	}
	app.logger.Debug().Msg("message sending")
	// Use the basic version.
	if app.vanilla {
		partition, err := hash(input.Key, app.conf.Partitions)
		if err != nil {
			app.logger.Error().AnErr(fmt.Sprintf("Kafka hashing error: %s", input.Key), err)
			return
		}
		app.logger.Printf("Kafka: Hashing new key to partition %d of %d partitions.", partition, app.conf.Partitions)
		go app.Produce(input.Key, input.Body, partition)
	} else { // Use the SLOPS algorithm.
		var partition int32
		if rec := app.partitionMap.GetKey(input.Key); rec == nil { // Use KeyMap to decide partition.
			partition, err = hash(input.Key, app.conf.Partitions)
			if err != nil {
				app.logger.Error().AnErr(fmt.Sprintf("SMALOPS hashing error: %s", input.Key), err)
				return
			}
			app.logger.Printf("SMALOPS: Hashing new key to partition %d of %d partitions.", partition, app.conf.Partitions)
		} else {
			app.logger.Printf("SMALOPS: Sending to partition %d of %d partitions.", rec.Partition, app.conf.Partitions)
			partition = int32(rec.Partition)
			// Message Set header will be added by `Producer` when message is sent.
		}
		go app.Produce(input.Key, input.Body, partition)
	}

	app.logger.Debug().Str("Received new request:", input.String())
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
