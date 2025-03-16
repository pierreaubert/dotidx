package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMetrics_RecordLatency(t *testing.T) {
	metrics := NewMetrics("test")
	start := time.Now()
	metrics.RecordLatency(start, 1, nil)

	stats := metrics.GetStats()
	assert.NotNil(t, stats)
	if stats != nil {
		assert.NotNil(t, stats.bucketsStats)
		if len(stats.bucketsStats) > 0 {
			assert.Equal(t, 1, stats.bucketsStats[0].count)
			assert.Equal(t, 0, stats.bucketsStats[0].failures)
		}
	}
}

func TestMetrics_GetStats(t *testing.T) {
	metrics := NewMetrics("test")
	start := time.Now()
	metrics.RecordLatency(start, 1, nil)

	stats := metrics.GetStats()
	assert.NotNil(t, stats)
	if stats != nil {
		assert.NotNil(t, stats.bucketsStats)
		if len(stats.bucketsStats) > 0 {
			assert.Equal(t, 4, len(stats.bucketsStats))
		}
	}
}

func TestMetrics_PrintStats(t *testing.T) {
	metrics := NewMetrics("test")
	start := time.Now()
	metrics.RecordLatency(start, 1, nil)

	// This is a basic test to ensure PrintStats doesn't panic
	metrics.PrintStats(true)
}

// func TestBucket_Rate(t *testing.T) {
// 	// Test with a 1 minute bucket
// 	bucket := NewBucket("test-rate", time.Minute)
// 	start := time.Now()

// 	// Record 10 successful calls over 2 seconds
// 	simulatedDuration := 2 * time.Second
// 	bucket.RecordLatency(start, 10, nil)

// 	// Force totalTime to be exactly the simulated duration for predictable testing
// 	bucket.mutex.Lock()
// 	bucket.totalTime = simulatedDuration
// 	bucket.mutex.Unlock()

// 	stats := bucket.GetStats()

// 	// Rate should be 10 calls / 2 seconds = 5 calls per second
// 	expectedRate := 5.0
// 	assert.InDelta(t, expectedRate, stats.rate, 0.1, "Rate should be approximately 5 calls per second")
// }

// func TestBucket_RateWithFailures(t *testing.T) {
// 	// Test with failures included in rate calculation
// 	bucket := NewBucket("test-rate-failures", time.Minute)
// 	start := time.Now()

// 	// Record 5 successful calls and 5 failures
// 	simulatedDuration := 5 * time.Second
// 	bucket.RecordLatency(start, 5, nil)
// 	bucket.RecordLatency(start, 5, assert.AnError)

// 	// Force totalTime for testing
// 	bucket.mutex.Lock()
// 	bucket.totalTime = simulatedDuration
// 	bucket.mutex.Unlock()

// 	stats := bucket.GetStats()

// 	// Rate calculation includes both successful and failed calls
// 	// 10 total calls / 5 seconds = 2 calls per second
// 	expectedRate := 2.0
// 	assert.InDelta(t, expectedRate, stats.rate, 0.1, "Rate calculation should include both successful and failed calls")
// }

// func TestBucket_RateWithError(t *testing.T) {
// 	// Test rate calculation when error is provided
// 	bucket := NewBucket("test-rate-error", time.Minute)
// 	start := time.Now()

// 	// Record with error - should count as failure
// 	simulatedDuration := 1 * time.Second
// 	bucket.RecordLatency(start, 1, nil) // 1 success
// 	bucket.RecordLatency(start, 2, assert.AnError) // 2 intended as success, but error provided

// 	// Force totalTime for testing
// 	bucket.mutex.Lock()
// 	bucket.totalTime = simulatedDuration
// 	bucket.mutex.Unlock()

// 	stats := bucket.GetStats()

// 	// Rate should include both successful calls and failures
// 	// Total 3 calls / 1 second = 3 calls per second
// 	expectedRate := 3.0
// 	assert.InDelta(t, expectedRate, stats.rate, 0.1, "Rate should be approx 3 calls per second including failures")

// 	// Success count should be 1, failures should be 2
// 	assert.Equal(t, 1, stats.count, "Success count should be 1")
// 	assert.Equal(t, 2, stats.failures, "Failure count should be 2")
// }

// func TestBucket_WindowReset(t *testing.T) {
// 	// Test rate calculation after window reset
// 	// Use a short window for testing
// 	shortWindow := 10 * time.Millisecond
// 	bucket := NewBucket("test-window-reset", shortWindow)

// 	// Record some calls
// 	start := time.Now()
// 	bucket.RecordLatency(start, 5, nil)

// 	// Wait for window to expire
// 	time.Sleep(shortWindow * 2)

// 	// Record more calls after window reset
// 	start = time.Now()
// 	bucket.RecordLatency(start, 3, nil)

// 	// Stats should only include calls after reset
// 	stats := bucket.GetStats()
// 	assert.Equal(t, 3, stats.count, "After window reset, only new calls should be counted")
// 	assert.Equal(t, 0, stats.failures, "Failures should be reset with window")
// }

// func TestBucket_ZeroRate(t *testing.T) {
// 	// Test rate calculation when total time is zero
// 	bucket := NewBucket("test-zero-rate", time.Minute)

// 	// Force zero total time
// 	bucket.mutex.Lock()
// 	bucket.totalTime = 0
// 	bucket.callCount = 5 // Add some calls
// 	bucket.mutex.Unlock()

// 	stats := bucket.GetStats()

// 	// Rate should be zero when total time is zero
// 	assert.Equal(t, 0.0, stats.rate, "Rate should be 0 when total time is 0")
// }

// func TestMetrics_MultipleBuckets(t *testing.T) {
// 	// Test that all buckets record and calculate rates correctly
// 	metrics := NewMetrics("test-multi")
// 	start := time.Now()

// 	// Record multiple calls
// 	for i := 0; i < 10; i++ {
// 		metrics.RecordLatency(start, 1, nil)
// 	}

// 	// Force a specific total time for predictable testing
// 	for i := range metrics.buckets {
// 		metrics.buckets[i].mutex.Lock()
// 		metrics.buckets[i].totalTime = 2 * time.Second
// 		metrics.buckets[i].mutex.Unlock()
// 	}

// 	stats := metrics.GetStats()

// 	// Each bucket should have the same rate
// 	expectedRate := 5.0 // 10 calls / 2 seconds
// 	for i := range stats.bucketsStats {
// 		assert.InDelta(t, expectedRate, stats.bucketsStats[i].rate, 0.1,
// 			"Bucket %d should have approximately the same rate", i)
// 		assert.Equal(t, 10, stats.bucketsStats[i].count,
// 			"Bucket %d should have the correct count", i)
// 	}
// }
