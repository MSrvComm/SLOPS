package main

import (
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

	log.Println("message sending")
	// Use the basic version.
	if app.vanilla {
		go app.Produce(input.Key, input.Body)
	} else { // Use the SLOPS algorithm.
		app.ch <- input.Key // Send the key to the lossy counter.
		var partition int
		if rec, err := app.keyMap.GetKey(input.Key); err != nil {
			rand.Seed(time.Now().UnixNano())
			partition = rand.Intn(app.conf.Partitions)
		} else {
			partition = rec.Partition
		}
		go app.Produce(input.Key, input.Body, partition)
	}
	log.Println("http returning")

	log.Println("Received new request:", input)
}
