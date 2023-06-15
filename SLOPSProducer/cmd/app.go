package main

import (
	"log"
	"os"

	"github.com/MSrvComm/SLOPSProducer/internal"
)

type Application struct {
	vanilla      bool                    // If true then do not use the SLOPS algorithm.
	ch           chan string             // Receive incoming keys through this channel.
	conf         *internal.Config        // Hold the configuration data.
	partitionMap *internal.PartitionMap  // Hot keys mapped to each partition.
	messageSets  *internal.MessageSetMap // Map Message Sets
	logger       *log.Logger             // System level logger.
	producer     Producer                // Kafka producer.
}

func NewApp(vanilla bool, conf *internal.Config) *Application {
	return &Application{
		vanilla:      vanilla,
		ch:           make(chan string),
		conf:         conf,
		partitionMap: internal.NewPartitionMap(),
		messageSets:  &internal.MessageSetMap{KV: map[string]internal.MessageSet{}},
		logger:       log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Llongfile),
	}
}
