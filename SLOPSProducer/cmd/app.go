package main

import (
	"os"

	"github.com/MSrvComm/SLOPSProducer/configs"
	"github.com/MSrvComm/SLOPSProducer/internal"
	"github.com/rs/zerolog"
)

type Application struct {
	vanilla      bool                      // If true then do not use the SLOPS algorithm.
	ch           chan string               // Receive incoming keys through this channel.
	conf         *configs.Config           // Hold the configuration data.
	partitionMap *internal.PartitionMap  // Hot keys mapped to each partition.
	messageSets  *internal.MessageSetMap // Map Message Sets
	logger       zerolog.Logger            // System level logger.
	producer     Producer                  // Kafka producer.
}

func NewApp(vanilla bool, conf *configs.Config) *Application {
	return &Application{
		vanilla: vanilla,
		ch:      make(chan string),
		conf:    conf,
		partitionMap: internal.NewPartitionMap(conf.SwapInterval),
		messageSets:  &internal.MessageSetMap{KV: map[string]internal.MessageSet{}},
		logger:       zerolog.New(os.Stdout).With().Timestamp().Logger(),
	}
}
