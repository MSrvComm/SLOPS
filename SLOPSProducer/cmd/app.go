package main

import (
	"log"
	"sync"

	"github.com/MSrvComm/SLOPSProducer/internal"
)

type Application struct {
	sync.Mutex
	ch               chan string
	conf             internal.Config
	keyMap           *internal.KeyMap
	partitionWeights []float64
	logger           *log.Logger
	seq              *Sequencer
	traceUrl         string
	producer         Producer
}
