package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/lib/pq"
)

func TestParseFlags(t *testing.T) {
	// Create a temporary function that mimics parseFlags but uses a new FlagSet
	parseTestFlags := func(args []string) Config {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		startRange := fs.Int("start", 1, "Start of the integer range")
		endRange := fs.Int("end", 10, "End of the integer range")
		sidecarURL := fs.String("sidecar", "", "Sidecar URL")
		postgresURI := fs.String("postgres", "", "PostgreSQL connection URI")
		batchSize := fs.Int("batch", 100, "Number of items to collect before writing to database")
		maxWorkers := fs.Int("workers", 5, "Maximum number of concurrent workers")
		flushTimeout := fs.Duration("flush", 30*time.Second, "Maximum time to wait before flushing data to database")

		// Parse the provided args (skip the program name)
		fs.Parse(args[1:])

		return Config{
			StartRange:   *startRange,
			EndRange:     *endRange,
			SidecarURL:   *sidecarURL,
			PostgresURI:  *postgresURI,
			BatchSize:    *batchSize,
			MaxWorkers:   *maxWorkers,
			FlushTimeout: *flushTimeout,
		}
	}

	tests := []struct {
		name     string
		args     []string
		expected Config
	}{
		{
			name: "Default values",
			args: []string{"cmd"},
			expected: Config{
				StartRange:   1,
				EndRange:     10,
				SidecarURL:   "",
				PostgresURI:  "",
				BatchSize:    100,
				MaxWorkers:   5,
				FlushTimeout: 30 * time.Second,
			},
		},
		{
			name: "Custom values",
			args: []string{
				"cmd",
				"-start=5",
				"-end=15",
				"-sidecar=http://example.com/sidecar",
				"-postgres=postgres://user:pass@localhost:5432/db",
				"-batch=50",
				"-workers=10",
				"-flush=1m",
			},
			expected: Config{
				StartRange:   5,
				EndRange:     15,
				SidecarURL:   "http://example.com/sidecar",
				PostgresURI:  "postgres://user:pass@localhost:5432/db",
				BatchSize:    50,
				MaxWorkers:   10,
				FlushTimeout: 1 * time.Minute,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := parseTestFlags(tc.args)

			if config.StartRange != tc.expected.StartRange {
				t.Errorf("Expected StartRange=%d, got %d", tc.expected.StartRange, config.StartRange)
			}
			if config.EndRange != tc.expected.EndRange {
				t.Errorf("Expected EndRange=%d, got %d", tc.expected.EndRange, config.EndRange)
			}
			if config.SidecarURL != tc.expected.SidecarURL {
				t.Errorf("Expected SidecarURL=%s, got %s", tc.expected.SidecarURL, config.SidecarURL)
			}
			if config.PostgresURI != tc.expected.PostgresURI {
				t.Errorf("Expected PostgresURI=%s, got %s", tc.expected.PostgresURI, config.PostgresURI)
			}
			if config.BatchSize != tc.expected.BatchSize {
				t.Errorf("Expected BatchSize=%d, got %d", tc.expected.BatchSize, config.BatchSize)
			}
			if config.MaxWorkers != tc.expected.MaxWorkers {
				t.Errorf("Expected MaxWorkers=%d, got %d", tc.expected.MaxWorkers, config.MaxWorkers)
			}
			if config.FlushTimeout != tc.expected.FlushTimeout {
				t.Errorf("Expected FlushTimeout=%v, got %v", tc.expected.FlushTimeout, config.FlushTimeout)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name: "Valid config",
			config: Config{
				StartRange:   1,
				EndRange:     10,
				SidecarURL:   "http://example.com/sidecar",
				PostgresURI:  "postgres://user:pass@localhost:5432/db",
				BatchSize:    100,
				MaxWorkers:   5,
				FlushTimeout: 30 * time.Second,
			},
			expectError: false,
		},
		{
			name: "Start range greater than end range",
			config: Config{
				StartRange:   10,
				EndRange:     1,
				SidecarURL:   "http://example.com/sidecar",
				PostgresURI:  "postgres://user:pass@localhost:5432/db",
				BatchSize:    100,
				MaxWorkers:   5,
				FlushTimeout: 30 * time.Second,
			},
			expectError: true,
		},
		{
			name: "Missing Sidecar URL",
			config: Config{
				StartRange:   1,
				EndRange:     10,
				SidecarURL:   "",
				PostgresURI:  "postgres://user:pass@localhost:5432/db",
				BatchSize:    100,
				MaxWorkers:   5,
				FlushTimeout: 30 * time.Second,
			},
			expectError: true,
		},
		{
			name: "Missing PostgreSQL URI",
			config: Config{
				StartRange:   1,
				EndRange:     10,
				SidecarURL:   "http://example.com/sidecar",
				PostgresURI:  "",
				BatchSize:    100,
				MaxWorkers:   5,
				FlushTimeout: 30 * time.Second,
			},
			expectError: true,
		},
		{
			name: "Invalid batch size",
			config: Config{
				StartRange:   1,
				EndRange:     10,
				SidecarURL:   "http://example.com/sidecar",
				PostgresURI:  "postgres://user:pass@localhost:5432/db",
				BatchSize:    0,
				MaxWorkers:   5,
				FlushTimeout: 30 * time.Second,
			},
			expectError: true,
		},
		{
			name: "Invalid max workers",
			config: Config{
				StartRange:   1,
				EndRange:     10,
				SidecarURL:   "http://example.com/sidecar",
				PostgresURI:  "postgres://user:pass@localhost:5432/db",
				BatchSize:    100,
				MaxWorkers:   0,
				FlushTimeout: 30 * time.Second,
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfig(tc.config)
			if (err != nil) != tc.expectError {
				t.Errorf("Expected error=%v, got error=%v", tc.expectError, err != nil)
			}
		})
	}
}

func TestCallSidecar(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the request URL matches the expected pattern
		if r.URL.Path != "/blocks/1" {
			t.Errorf("Expected request to '/blocks/1', got '%s'", r.URL.Path)
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		// Return a mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{
			"timestamp": "2023-01-01T00:00:00Z",
			"hash": "0x1234567890abcdef",
			"parenthash": "0xabcdef1234567890",
			"stateroot": "0x1234567890abcdef1234567890abcdef",
			"extrinsicsroot": "0xabcdef1234567890abcdef1234567890",
			"authorid": "0x1234567890",
			"finalized": true,
			"oninitialize": {},
			"onfinalize": {},
			"logs": [],
			"extrinsics": []
		}`)
	}))
	defer server.Close()

	// Create a context
	ctx := context.Background()

	// Call the function
	blockData, err := fetchData(ctx, 1, server.URL)
	if err != nil {
		t.Fatalf("fetchData returned an error: %v", err)
	}

	// Check the result
	if blockData.ID != 1 {
		t.Errorf("Expected ID=1, got %d", blockData.ID)
	}
	if blockData.Hash != "0x1234567890abcdef" {
		t.Errorf("Expected Hash=0x1234567890abcdef, got %s", blockData.Hash)
	}
	if blockData.ParentHash != "0xabcdef1234567890" {
		t.Errorf("Expected ParentHash=0xabcdef1234567890, got %s", blockData.ParentHash)
	}
	if !blockData.Finalized {
		t.Errorf("Expected Finalized=true, got %v", blockData.Finalized)
	}
}

func TestSaveToDatabase(t *testing.T) {
	// Skip if TEST_POSTGRES_URI is not set
	postgresURI := os.Getenv("TEST_POSTGRES_URI")
	if postgresURI == "" {
		t.Skip("Skipping database test. Set TEST_POSTGRES_URI environment variable to run.")
	}

	// Ensure sslmode=disable is in the PostgreSQL URI if not already present
	if !strings.Contains(postgresURI, "sslmode=") {
		if strings.Contains(postgresURI, "?") {
			postgresURI += "&sslmode=disable"
		} else {
			postgresURI += "?sslmode=disable"
		}
	}

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", postgresURI)
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	// Create table
	if err := createTable(db); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Clean up test data
	_, err = db.Exec("DELETE FROM blocks_Polkadot_Polkadot WHERE hash = $1 OR hash = $2", "0x1234567890abcdef", "0xabcdef1234567890")
	if err != nil {
		t.Fatalf("Failed to clean up test data: %v", err)
	}

	// Create test data
	now := time.Now()
	items := []BlockData{
		{
			ID:             1, // Used for sidecar API call
			Timestamp:      now,
			Hash:           "0x1234567890abcdef",
			ParentHash:     "0x0987654321fedcba",
			StateRoot:      "0xabcdef1234567890",
			ExtrinsicsRoot: "0xfedcba0987654321",
			AuthorID:       "0xaabbccddeeff",
			Finalized:      true,
			OnInitialize:   json.RawMessage(`{"events":[]}`),
			OnFinalize:     json.RawMessage(`{"events":[]}`),
			Logs:           json.RawMessage(`["log1", "log2"]`),
			Extrinsics:     json.RawMessage(`["ex1", "ex2"]`),
		},
		{
			ID:             2, // Used for sidecar API call
			Timestamp:      now.Add(time.Second),
			Hash:           "0xabcdef1234567890",
			ParentHash:     "0x1234567890abcdef",
			StateRoot:      "0x0987654321fedcba",
			ExtrinsicsRoot: "0xaabbccddeeff",
			AuthorID:       "0xfedcba0987654321",
			Finalized:      false,
			OnInitialize:   json.RawMessage(`{"events":[]}`),
			OnFinalize:     json.RawMessage(`{"events":[]}`),
			Logs:           json.RawMessage(`["log3", "log4"]`),
			Extrinsics:     json.RawMessage(`["ex3", "ex4"]`),
		},
	}
	
	// Save to database
	err = saveToDatabase(db, items)
	if err != nil {
		t.Fatalf("Failed to save to database: %v", err)
	}
	
	// Verify data was saved
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM blocks_Polkadot_Polkadot").Scan(&count)
	if err != nil {
		t.Errorf("Failed to query database: %v", err)
	}
	
	if count != 2 {
		t.Errorf("Expected 2 records, got %d", count)
	}
	
	// Verify specific fields
	var hash string
	err = db.QueryRow("SELECT hash FROM blocks_Polkadot_Polkadot WHERE block_id = $1", 1).Scan(&hash)
	if err != nil {
		t.Errorf("Failed to query specific record: %v", err)
	}
	
	if hash != "0x1234567890abcdef" {
		t.Errorf("Expected hash=0x1234567890abcdef, got hash=%s", hash)
	}
}

func TestCreateTable(t *testing.T) {
	// Create a test database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	// Set up expectations
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS blocks_Polkadot_Polkadot").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Call the function
	if err := createTable(db); err != nil {
		t.Errorf("Failed to create table: %v", err)
	}

	// Check expectations
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestWorkerDistribution(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract block ID from URL
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 3 || parts[1] != "blocks" {
			t.Errorf("Unexpected URL path: %s", r.URL.Path)
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		
		// Return a mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{
			"timestamp": "2023-01-01T00:00:00Z",
			"hash": "0x%s",
			"parenthash": "0xabcdef1234567890",
			"stateroot": "0x1234567890abcdef1234567890abcdef",
			"extrinsicsroot": "0xabcdef1234567890abcdef1234567890",
			"authorid": "0x1234567890",
			"finalized": true,
			"oninitialize": {},
			"onfinalize": {},
			"logs": [],
			"extrinsics": []
		}`, parts[2]) // Use block ID in hash
	}))
	defer server.Close()

	// Create a test database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	// Set up expectations
	mock.ExpectQuery("SELECT block_id FROM blocks_Polkadot_Polkadot WHERE block_id BETWEEN \\$1 AND \\$2").
		WithArgs(1, 5).
		WillReturnRows(sqlmock.NewRows([]string{"block_id"}).AddRow(2).AddRow(4)) // Blocks 2 and 4 already exist

	// Expect transaction for batch insert
	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO blocks_Polkadot_Polkadot.*")
	
	// Expect a single exec with any arguments
	mock.ExpectExec("INSERT INTO blocks_Polkadot_Polkadot.*").
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	
	mock.ExpectExec("INSERT INTO blocks_Polkadot_Polkadot.*").
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	
	mock.ExpectExec("INSERT INTO blocks_Polkadot_Polkadot.*").
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	
	mock.ExpectCommit()

	// Create a test config
	config := Config{
		StartRange:   1,
		EndRange:     5,
		SidecarURL:   server.URL,
		PostgresURI:  "mock",
		BatchSize:    10,
		MaxWorkers:   2,
		FlushTimeout: 100 * time.Millisecond,
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Run the function
	startWorkers(ctx, config, db)

	// Check expectations
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestWorkerDistributionAllBlocksExist(t *testing.T) {
	// Create a test database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	// Set up expectations - all blocks already exist
	mock.ExpectQuery("SELECT block_id FROM blocks_Polkadot_Polkadot WHERE block_id BETWEEN \\$1 AND \\$2").
		WithArgs(1, 5).
		WillReturnRows(sqlmock.NewRows([]string{"block_id"}).
			AddRow(1).AddRow(2).AddRow(3).AddRow(4).AddRow(5)) // All blocks exist

	// Create a test config
	config := Config{
		StartRange:   1,
		EndRange:     5,
		SidecarURL:   "http://example.com",
		PostgresURI:  "mock",
		BatchSize:    10,
		MaxWorkers:   2,
		FlushTimeout: 100 * time.Millisecond,
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Run the function
	startWorkers(ctx, config, db)

	// Check expectations
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

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
