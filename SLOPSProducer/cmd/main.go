package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/MSrvComm/SLOPSProducer/internal"
)

type Input struct {
	Key string `json:"key"`
}

func main() {
	// Get the configuration data.
	data, err := os.ReadFile("/etc/producer/config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	var conf internal.Config
	if err := conf.Parse(data); err != nil {
		log.Fatal(err)
	}

	// var updateUnit time.Duration
	// if conf.UpdateIntervalUnit == "second" {
	// 	updateUnit = time.Second
	// } else if conf.UpdateIntervalUnit == "minute" {
	// 	updateUnit = time.Minute
	// } else {
	// 	log.Fatal("can only accept 'second' or 'minute' for interval unit.")
	// }

	// selfIP := os.Getenv("ADDRESS")
	// if selfIP == "" {
	// 	log.Fatal("not a valid IP")
	// }

	waitGroup := &sync.WaitGroup{}

	app := Application{ch: make(chan string),
		keyMap: &internal.KeyMap{KV: make(map[string]internal.KeyRecord)},
		logger: log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Llongfile),
	}

	// Start the lossy count thread.
	waitGroup.Add(1)
	go app.LossyCount(waitGroup)

	// Start Kafka producer.
	app.producer = app.NewProducer()
	successes := 0
	errors := 0

	// Handle signals.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	// Successes channel needs to be consumed for producer to run smoothly.
	waitGroup.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		for range app.producer.kafkaProducer.Successes() {
			successes++
		}
	}(waitGroup)

	// Errors channel needs to be consumed for producer to run smoothly.
	waitGroup.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		for err := range app.producer.kafkaProducer.Errors() {
			log.Println(err)
			errors++
		}
	}(waitGroup)

	// HTTP Server.
	srv := http.Server{
		Addr:         fmt.Sprintf(":%d", conf.HTTPPort),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Printf("Starting HTTP server on %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())
	waitGroup.Wait()
}
