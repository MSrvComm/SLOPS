package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
)

type Sequencer struct {
	sync.Mutex
	seq uint64
}

func NewSequencer() *Sequencer {
	return &Sequencer{seq: 1}
}

func (s *Sequencer) Next() uint64 {
	s.Lock()
	defer s.Unlock()
	v := s.seq
	s.seq++
	return v
}

func (app *Application) SendSequence(seq string) {
	url := fmt.Sprintf("%s/%s", app.traceUrl, seq)
	resp, err := http.Get(url)
	if err != nil {
		log.Println("Failed to connect to:", url)
		return
	}
	log.Printf("Response status %d from url %s\n", resp.StatusCode, url)
}
