package main

import (
	"log"
	"sync"
	"time"
)

// Bucket tracks performance metrics for sidecar API calls
type Bucket struct {
	mutex     sync.Mutex
	callCount int
	totalTime time.Duration
	minTime   time.Duration
	maxTime   time.Duration
	failures  int
	name      string
	window    time.Duration
	startedAt time.Time
}

type BucketStats struct {
	count, failures int
	avg, min, max   time.Duration
	rate            float64
}

// NewBucket creates a new Bucket instance
func NewBucket(name string, window time.Duration) *Bucket {
	return &Bucket{
		minTime:   time.Hour, // Initialize with a large value
		name:      name,
		window:    window,
		startedAt: time.Now(),
	}
}

func NewBucketStats() BucketStats {
	return BucketStats{}
}

// RecordLatency records the latency of a sidecar API call
func (m *Bucket) RecordLatency(start time.Time, countOK int, countError int, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	duration := time.Since(start)

	current := time.Since(m.startedAt)
	if current >= m.window {
		m.callCount = 0
		m.failures = 0
		m.totalTime = 0
		m.minTime = m.window + time.Duration(time.Minute)
		m.startedAt = time.Now()
	}
	if err != nil {
		m.callCount += countError
		m.failures += countOK
	} else {
		m.callCount += countOK
		m.failures += countError
	}
	m.totalTime += duration

	if count := int64(countOK + countError); count > 0 {
		relativeDuration := time.Duration(int64(duration) / count)
		m.minTime = min(relativeDuration, m.minTime)
		m.maxTime = max(relativeDuration, m.maxTime)
	}
}

// GetStats returns the current metrics statistics
func (m *Bucket) GetStats() (bs BucketStats) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	bs.count = m.callCount
	bs.failures = m.failures

	if bs.count > 0 {
		bs.avg = (m.totalTime / time.Duration(bs.count)).Round(time.Millisecond)
		bs.min = m.minTime.Round(time.Millisecond)
		bs.max = m.maxTime.Round(time.Millisecond)
		delta := time.Since(m.startedAt)
		if tt := delta.Milliseconds(); tt > 0 {
			bs.rate = float64(bs.count) * float64(1000.0) / float64(tt)
		}
	}

	return
}

// PrintStats prints the current metrics statistics
func (m *Bucket) PrintStats(printHeader bool) {
	bs := m.GetStats()
	if printHeader {
		log.Printf("Statistics: Total calls: %d failure: %d rate: %.1fs", bs.count, bs.failures, bs.rate)
	}
	if bs.count > 0 {
		log.Printf("  Latency avg: %v min: %v max: %v Success rate: %.2f%%",
			bs.avg, bs.min, bs.max, float64(bs.count)/(float64(bs.count+bs.failures))*100)
	}
}

// Metrics tracks performance metrics for API calls
type Metrics struct {
	buckets []*Bucket
}

type MetricsStats struct {
	bucketsStats [4]BucketStats
}

// NewMetrics creates a new Metrics instance
func NewMetrics(name string) *Metrics {
	return &Metrics{
		buckets: []*Bucket{
			NewBucket(name, time.Duration(time.Hour*24)),
			NewBucket(name, time.Duration(time.Hour)),
			NewBucket(name, time.Duration(time.Minute*5)),
			NewBucket(name, time.Duration(time.Minute)),
		},
	}
}

// RecordLatency records the latency of a sidecar API call
func (m *Metrics) RecordLatency(start time.Time, countOK int, countError int, err error) {
	for i := range m.buckets {
		m.buckets[i].RecordLatency(start, countOK, countError, err)
	}
}

func NewMetricsStats() *MetricsStats {
	return &MetricsStats{
		bucketsStats: [4]BucketStats{
			NewBucketStats(),
			NewBucketStats(),
			NewBucketStats(),
			NewBucketStats(),
		},
	}
}

// GetStats returns the current metrics statistics
func (m *Metrics) GetStats() (s *MetricsStats) {
	s = NewMetricsStats()
	for i := range m.buckets {
		s.bucketsStats[i] = m.buckets[i].GetStats()
	}
	return
}

// PrintStats prints the current metrics statistics
func (m *Metrics) PrintStats(printHeader bool) {
	for i := range m.buckets {
		m.buckets[i].PrintStats(printHeader)
	}
}
