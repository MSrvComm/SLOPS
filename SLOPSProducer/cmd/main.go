package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/MSrvComm/SLOPSProducer/internal"
	"github.com/rs/zerolog"
)

func main() {
	// Get configuration data.
	data, err := os.ReadFile("/etc/producer/config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	var conf internal.Config
	if err := conf.Parse(data); err != nil {
		log.Fatal(err)
	}

	vanilla, err := strconv.ParseBool(os.Getenv("VANILLA"))
	if err != nil {
		log.Fatal(err)
	}

	wg := &sync.WaitGroup{}

	app := NewApp(vanilla, &conf)

	if os.Getenv("ENV") == "dev" {
		app.logger.Level(zerolog.DebugLevel)
	} else {
		app.logger.Level(zerolog.InfoLevel)
	}

	// Populate partitions in partition map.
	app.partitionMap.PopulateMaps(int(app.conf.Partitions))

	// Start the Kafka producer.
	app.producer = app.NewProducer()
	successes := 0
	errors := 0

	// Handle signals.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	// Successes channel needs to be consumed for producer to run smoothly.
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		for s := range app.producer.kafkaProducer.Successes() {
			// Print out timestamp, partition and offset.
			// Later we will use this to realize total rate of messages into a partition.
			app.logger.Info().Msgf("Received Offset: %d at time %v on partition %d", s.Offset, s.Timestamp, s.Partition)
			successes++
		}
	}(wg)

	// Errors channel needs to be consumed for producer to run smoothly.
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		for err := range app.producer.kafkaProducer.Errors() {
			app.logger.Error().AnErr("Kafka Error", err)
			errors++
		}
	}(wg)

	// We want to track the partition weights for basic Kafka as well.
	wg.Add(1)
	go app.LossyCount(wg)

	// And print out the weights every second.
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		logTicker := time.NewTicker(time.Second)
		for range logTicker.C {
			partitionSizes := make([]float64, app.conf.Partitions)
			for p := 0; p < int(app.conf.Partitions); p++ {
				partitionSizes[p] = app.partitionMap.PartitionSize(p)
			}
			// app.logger.Println("Partition Weights:", partitionSizes)
			app.logger.Debug().Str("Partition Weights:", fmt.Sprintf("%v", partitionSizes))
		}
	}(wg)

	// Swap stores if SMALOPS is being used.
	if !app.vanilla {
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			swapTicker := time.NewTicker(time.Second * time.Duration(app.conf.SwapInterval))
			for range swapTicker.C {
				app.partitionMap.Rebalance()
			}
		}(wg)
	}

	// HTTP Server.
	srv := http.Server{
		Addr:         fmt.Sprintf(":%d", conf.HTTPPort),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	app.logger.Info().Msg(fmt.Sprintf("Starting HTTP server on %s", srv.Addr))
	app.logger.Fatal().AnErr("server failure", srv.ListenAndServe())
	wg.Wait()
}
