package dotidx

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
		assert.NotNil(t, stats.BucketsStats)
		if len(stats.BucketsStats) > 0 {
			assert.Equal(t, 1, stats.BucketsStats[0].Count)
			assert.Equal(t, 0, stats.BucketsStats[0].Failures)
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
		assert.NotNil(t, stats.BucketsStats)
		if len(stats.BucketsStats) > 0 {
			assert.Equal(t, 4, len(stats.BucketsStats))
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

