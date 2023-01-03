package main

import (
	"log"

	"github.com/MSrvComm/SLOPSProducer/internal"
)

type Application struct {
	ch       chan string
	keyMap   *internal.KeyMap
	logger   *log.Logger
	producer Producer
}
