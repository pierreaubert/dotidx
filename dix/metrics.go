package dix

import (
	"log"
	"sort"
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
	Count, Failures int
	Avg, Min, Max   time.Duration
	Rate            float64
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

func (m *Bucket) RecordLatency(start time.Time, count int, err error) {
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
		m.failures += count
	} else {
		m.callCount += count
	}
	m.totalTime += duration

	if count > 0 {
		relativeDuration := time.Duration(int64(duration) / int64(count))
		m.minTime = min(relativeDuration, m.minTime)
		m.maxTime = max(relativeDuration, m.maxTime)
	}
}

func (m *Bucket) GetStats() (bs BucketStats) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	bs.Count = m.callCount
	bs.Failures = m.failures

	if bs.Count > 0 {
		bs.Avg = (m.totalTime / time.Duration(bs.Count)).Round(time.Millisecond)
		bs.Min = m.minTime.Round(time.Millisecond)
		bs.Max = m.maxTime.Round(time.Millisecond)
		delta := time.Since(m.startedAt)
		if tt := delta.Milliseconds(); tt > 0 {
			bs.Rate = float64(bs.Count) * float64(1000.0) / float64(tt)
		}
	}

	return
}

func (bs BucketStats) RateSinceStart() (rate float64) {
	if bs.Count+bs.Failures > 0 {
		rate = float64(bs.Failures) / float64(bs.Count+bs.Failures) * 100
	}
	return
}

// PrintStats prints the current metrics statistics
func (m *Bucket) PrintStats(printHeader bool) {
	bs := m.GetStats()
	if printHeader {
		log.Printf("Statistics: Total calls: %d failure: %d rate: %.1fs", bs.Count, bs.Failures, bs.Rate)
	}
	if bs.Count > 0 {
		log.Printf("  Latency avg: %v min: %v max: %v Success rate: %.2f%%",
			bs.Avg, bs.Min, bs.Max, float64(bs.Count)/(float64(bs.Count+bs.Failures))*100)
	}
}

// Metrics tracks performance metrics for API calls
type Metrics struct {
	Buckets []*Bucket
}

type MetricsStats struct {
	BucketsStats [4]BucketStats
}

// NewMetrics creates a new Metrics instance
func NewMetrics(name string) *Metrics {
	return &Metrics{
		Buckets: []*Bucket{
			NewBucket(name, time.Duration(time.Hour*24)),
			NewBucket(name, time.Duration(time.Hour)),
			NewBucket(name, time.Duration(time.Minute*5)),
			NewBucket(name, time.Duration(time.Minute)),
		},
	}
}

// RecordLatency records the latency of a sidecar API call
func (m *Metrics) RecordLatency(start time.Time, count int, err error) {
	for i := range m.Buckets {
		m.Buckets[i].RecordLatency(start, count, err)
	}
}

func NewMetricsStats() *MetricsStats {
	return &MetricsStats{
		BucketsStats: [4]BucketStats{
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
	for i := range m.Buckets {
		s.BucketsStats[i] = m.Buckets[i].GetStats()
	}
	return
}

// PrintStats prints the current metrics statistics
func (m *Metrics) PrintStats(printHeader bool) {
	for i := range m.Buckets {
		m.Buckets[i].PrintStats(printHeader)
	}
}

func CalculateAverageDurationFromSlice(durations []time.Duration) time.Duration {
	var total time.Duration
	if len(durations) == 0 {
		return 0
	}
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

func CalculatePercentile(durations []time.Duration, percentile int) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	// Create a copy to avoid modifying the original slice
	sortedDurations := make([]time.Duration, len(durations))
	copy(sortedDurations, durations)
	sort.Slice(sortedDurations, func(i, j int) bool {
		return sortedDurations[i] < sortedDurations[j]
	})
	pIndex := int(float64(len(sortedDurations))*float64(percentile)/100.0) - 1
	if pIndex < 0 {
		pIndex = 0 // Handle small sample sizes
	}
	if pIndex >= len(sortedDurations) { // Ensure index is within bounds
		pIndex = len(sortedDurations) - 1
	}
	return sortedDurations[pIndex]
}
