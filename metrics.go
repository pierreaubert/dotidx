package main

import (
	"log"
	"sync"
	"time"
)

// Metrics tracks performance metrics for sidecar API calls
type Metrics struct {
	mutex     sync.Mutex
	callCount int
	totalTime time.Duration
	minTime   time.Duration
	maxTime   time.Duration
	failures  int
	name      string
}

// NewMetrics creates a new Metrics instance
func NewMetrics(name string) *Metrics {
	return &Metrics{
		minTime: time.Hour, // Initialize with a large value
		name:    name,
	}
}

// RecordLatency records the latency of a sidecar API call
func (m *Metrics) RecordLatency(start time.Time, countOK int, countError int, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	duration := time.Since(start)
	if err != nil {
		m.callCount += countError
		m.failures += countOK
	} else {
		m.callCount += countOK
		m.failures += countError
	}
	m.totalTime += duration

	relativeDuration := time.Duration(int64(duration) / int64(countOK+countError))
	if relativeDuration < m.minTime {
		m.minTime = relativeDuration
	}

	if relativeDuration > m.maxTime {
		m.maxTime = relativeDuration
	}

}

// GetStats returns the current metrics statistics
func (m *Metrics) GetStats() (count int, avgTime, minTime, maxTime time.Duration, failures int, rate float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	count = m.callCount
	failures = m.failures

	if count > 0 {
		avgTime = (m.totalTime / time.Duration(count)).Round(time.Millisecond)
		minTime = m.minTime.Round(time.Millisecond)
		maxTime = m.maxTime.Round(time.Millisecond)
		rate = float64(count+failures) / m.totalTime.Seconds()
	}

	return
}

// PrintStats prints the current metrics statistics
func (m *Metrics) PrintStats(printHeader bool) {
	count, avgTime, minTime, maxTime, failures, rate := m.GetStats()
	if printHeader {
		log.Printf("Statistics: Total calls: %d failure: %d rate: %.1fs", count, failures, rate)
	}
	if count > 0 {
		log.Printf("  Latency avg: %v min: %v max: %v Success rate: %.2f%%",
			avgTime, minTime, maxTime, float64(count)/(float64(count+failures))*100)
	}
}
