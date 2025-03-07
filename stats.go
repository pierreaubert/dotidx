package main

import (
	"log"
	"sync"
	"time"
)

// SidecarMetrics tracks performance metrics for sidecar API calls
type SidecarMetrics struct {
	mutex     sync.Mutex
	callCount int
	totalTime time.Duration
	minTime   time.Duration
	maxTime   time.Duration
	failures  int
}

// NewSidecarMetrics creates a new SidecarMetrics instance
func NewSidecarMetrics() *SidecarMetrics {
	return &SidecarMetrics{
		minTime: time.Hour, // Initialize with a large value
	}
}

// RecordLatency records the latency of a sidecar API call
func (m *SidecarMetrics) RecordLatency(start time.Time, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if err != nil {
		m.failures++
		return
	}

	duration := time.Since(start)
	m.callCount++
	m.totalTime += duration

	if duration < m.minTime {
		m.minTime = duration
	}

	if duration > m.maxTime {
		m.maxTime = duration
	}
}

// GetStats returns the current metrics statistics
func (m *SidecarMetrics) GetStats() (count int, avgTime, minTime, maxTime time.Duration, failures int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	count = m.callCount
	failures = m.failures

	if count > 0 {
		avgTime = m.totalTime / time.Duration(count)
		minTime = m.minTime
		maxTime = m.maxTime
	}

	return
}

// PrintStats prints the current metrics statistics
func (m *SidecarMetrics) PrintStats() {
	count, avgTime, minTime, maxTime, failures := m.GetStats()

	if count == 0 && failures == 0 {
		log.Println("No sidecar API calls made")
		return
	}

	log.Printf("Sidecar API Call Statistics:")
	log.Printf("  Total calls: %d", count)
	log.Printf("  Failed calls: %d", failures)

	if count > 0 {
		log.Printf("  Average latency: %v", avgTime)
		log.Printf("  Minimum latency: %v", minTime)
		log.Printf("  Maximum latency: %v", maxTime)
		log.Printf("  Success rate: %.2f%%", float64(count)/(float64(count+failures))*100)
	}
}
