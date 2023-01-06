package main

import (
	"log"
	"sync"

	"github.com/MSrvComm/SLOPSProducer/internal"
)

type Application struct {
	sync.Mutex
	vanilla          bool // Do not use the key mapping.
	ch               chan string
	conf             internal.Config
	keyMap           *internal.KeyMap
	partitionWeights []float64
	logger           *log.Logger
	producer         Producer
}
