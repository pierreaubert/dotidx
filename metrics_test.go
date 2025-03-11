package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMetrics_RecordLatency(t *testing.T) {
	metrics := NewMetrics("test")
	start := time.Now()
	metrics.RecordLatency(start, 1, 0, nil)

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
	metrics.RecordLatency(start, 1, 0, nil)

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
	metrics.RecordLatency(start, 1, 0, nil)

	// This is a basic test to ensure PrintStats doesn't panic
	metrics.PrintStats(true)
}
