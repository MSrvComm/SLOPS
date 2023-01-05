package internal

import (
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"
)

type traceID string
type startTime time.Time

type OpenTraces struct {
	sync.RWMutex
	traces map[traceID]startTime
}

func NewOpenTraces() OpenTraces {
	return OpenTraces{
		traces: map[traceID]startTime{},
	}
}

func (t *OpenTraces) AddTrace(trace string) {
	t.Lock()
	defer t.Unlock()
	t.traces[traceID(trace)] = startTime(time.Now())
}

func (t *OpenTraces) GetTraceStartTime(trace string) (time.Time, error) {
	t.RLock()
	defer t.RUnlock()
	if val, exists := t.traces[traceID(trace)]; exists {
		return time.Time(val), nil
	}
	return time.Now(), errors.New("trace not found")
}

type CompletedJobs struct {
	sync.RWMutex
	jobs map[traceID]int64
}

func NewCompletedJobs() CompletedJobs {
	return CompletedJobs{
		jobs: map[traceID]int64{},
	}
}

func (c *CompletedJobs) AddJob(trace string, start time.Time) {
	c.Lock()
	defer c.Unlock()

	c.jobs[traceID(trace)] = time.Since(start).Microseconds()
}

func (c *CompletedJobs) GetCompletedJobs() ([]byte, error) {
	c.RLock()
	defer c.RUnlock()

	data, err := json.MarshalIndent(c.jobs, "", "\t")
	if err != nil {
		log.Println("Failed to print jobs:", err)
		return nil, err
	}
	data = append(data, '\n')
	return data, nil
}
