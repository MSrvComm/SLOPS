package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/MSrvComm/SLOPSTracer/internal"
	"github.com/gin-gonic/gin"
)

func main() {
	port := os.Getenv("PORT")
	server := Server{openTraces: internal.NewOpenTraces(), completedJobs: internal.NewCompletedJobs()}

	// HTTP Server.
	srv := http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      server.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Printf("Starting HTTP server on %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}

func (s *Server) routes() *gin.Engine {
	router := gin.Default()
	router.GET("/new/:traceID", s.NewTrace)
	router.GET("/close/:traceID", s.CloseTrace)
	router.GET("/all", s.GetAllJobs)
	return router
}

func (s *Server) NewTrace(c *gin.Context) {
	traceID := c.Param("traceID")
	s.OpenTrace(traceID)
}

func (s *Server) CloseTrace(c *gin.Context) {
	traceID := c.Param("traceID")
	s.CompleteJob(traceID)
}

func (s *Server) GetAllJobs(c *gin.Context) {
	bytes, err := s.completedJobs.GetCompletedJobs()
	if err != nil {
		log.Println(err)
		c.Writer.WriteHeader(500)
		return
	}
	w := c.Writer
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(bytes)
}
