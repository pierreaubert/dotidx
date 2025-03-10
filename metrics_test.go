package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/lib/pq"
)

func TestSidecarMetrics(t *testing.T) {
	// Create a new metrics instance
	m := NewSidecarMetrics()

	// Test with no calls
	count, avgTime, minTime, maxTime, failures := m.GetStats()
	if count != 0 || failures != 0 {
		t.Errorf("Expected 0 calls and 0 failures, got %d calls and %d failures", count, failures)
	}

	// Test with successful calls
	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	m.RecordLatency(start, nil)

	start = time.Now()
	time.Sleep(20 * time.Millisecond)
	m.RecordLatency(start, nil)

	count, avgTime, minTime, maxTime, failures = m.GetStats()
	if count != 2 {
		t.Errorf("Expected 2 calls, got %d", count)
	}
	if failures != 0 {
		t.Errorf("Expected 0 failures, got %d", failures)
	}
	if avgTime < 10*time.Millisecond || avgTime > 30*time.Millisecond {
		t.Errorf("Expected average time between 10ms and 30ms, got %v", avgTime)
	}
	if minTime > 20*time.Millisecond {
		t.Errorf("Expected minimum time <= 20ms, got %v", minTime)
	}
	if maxTime < 10*time.Millisecond {
		t.Errorf("Expected maximum time >= 10ms, got %v", maxTime)
	}

	// Test with failed calls
	m.RecordLatency(time.Now(), fmt.Errorf("test error"))

	count, _, _, _, failures = m.GetStats()
	if count != 2 {
		t.Errorf("Expected count to remain 2, got %d", count)
	}
	if failures != 1 {
		t.Errorf("Expected 1 failure, got %d", failures)
	}
}
