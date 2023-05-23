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

	vanilla, err := strconv.ParseBool(os.Getenv("VANILLA"))
	if err != nil {
		log.Fatal("Vanilla val not correct:", err)
	}
	p2c, err := strconv.ParseBool(os.Getenv("P2C"))
	if err != nil {
		log.Fatal("P2C val not correct:", err)
	}

	app := Application{
		vanilla: vanilla,
		p2c:     p2c,
		ch:      make(chan string),
		conf:    conf,
		// partitionMap:     &internal.PartitionMap{Keys: map[string]uint32{}},
		keyMap:           &internal.KeyMap{KV: make(map[string]internal.KeyRecord)},
		backupKeyMap:     &internal.KeyMap{KV: make(map[string]internal.KeyRecord)},
		messageSets:      &MessageSetMap{KV: map[string]MessageSet{}},
		partitionWeights: make([]float64, conf.Partitions),
		// randomPartitioner: internal.NewRandomPartitioner(),
		logger:       log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Llongfile),
		mapSwapTimer: *time.NewTicker(time.Duration(conf.SwapInterval) * time.Second),
	}

	// Populate the partitions in the partition map.
	app.partitionMap = &internal.PartitionMap{KV: map[int32][]internal.KeyCount{}, RebalanceMap: map[int32][]internal.KeyCount{}}
	for p := int32(0); p < app.conf.Partitions; p++ {
		app.partitionMap.KV[p] = []internal.KeyCount{}
	}

	// // Start the lossy count thread.
	// if !app.vanilla {
	// 	waitGroup.Add(1)
	// 	go app.LossyCount(waitGroup)
	// }

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
		for s := range app.producer.kafkaProducer.Successes() {
			// Print out timestamp, partition and offset.
			// Later we will use this to realize total rate of messages into a partition.
			log.Printf("Received Offset: %d at time %v on partition %d\n", s.Offset, s.Timestamp, s.Partition)
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

	// Start the map swap routine.
	if !app.vanilla {
		waitGroup.Add(2)
		go app.SwapMaps()
		go app.LossyCount(waitGroup)
	}

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
