package main

import (
	"log"
	"sync"

	"github.com/MSrvComm/SLOPSProducer/internal"
)

// Application is the workhorse of the system.
type Application struct {
	sync.Mutex
	vanilla           bool             // If true then do not use the SLOPS algorithm.
	p2c               bool             // If true then use P2C to load balance between partitions.
	ch                chan string      // Receive incoming keys through this channel.
	conf              internal.Config  // Hold the configuration data.
	keyMap            *internal.KeyMap // Mapping of hot keys.
	messageSets       *MessageSetMap   // Map Message Sets
	partitionWeights  []float64        // Weights of each partition.
	logger            *log.Logger      // System level logger.
	randomPartitioner *internal.RandomPartitioner
	producer          Producer // Kafka producer.
}
