package main

import (
	"log"

	"github.com/MSrvComm/SLOPSTracer/internal"
)

type Server struct {
	openTraces    internal.OpenTraces
	completedJobs internal.CompletedJobs
}

func (s *Server) OpenTrace(traceId string) {
	s.openTraces.AddTrace(traceId)
}

func (s *Server) CompleteJob(traceID string) {
	startTime, err := s.openTraces.GetTraceStartTime(traceID)
	if err != nil {
		log.Printf("Error %v found for trace %s\n", err, traceID)
	}

	s.completedJobs.AddJob(traceID, startTime)
}


