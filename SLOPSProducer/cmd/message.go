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
		if rec, err := app.keyMap.GetKey(input.Key); err != nil {
			go app.Produce(input.Key, input.Body)
		} else {
			go app.Produce(input.Key, input.Body, rec.Partition)
		}
	}
	log.Println("http returning")

	log.Println("Received new request:", input)
}
