package main

import (
	"log"

	"github.com/MSrvComm/SLOPSProducer/internal"
)

type Application struct {
	ch       chan string
	keyMap   *internal.KeyMap
	logger   *log.Logger
	seq      *Sequencer
	traceUrl string
	producer Producer
}
