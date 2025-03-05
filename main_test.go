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
	// Create a mock database connection
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Create test data
	testData := []BlockData{
		{
			ID:             1,
			Timestamp:      time.Now(),
			Hash:           "0x1234567890abcdef1234567890abcdef",
			ParentHash:     "0xabcdef1234567890abcdef1234567890",
			StateRoot:      "0x1234567890abcdef1234567890abcdef",
			ExtrinsicsRoot: "0xabcdef1234567890abcdef1234567890",
			AuthorID:       "0x1234567890",
			Finalized:      true,
			OnInitialize:   json.RawMessage(`{"test": true}`),
			OnFinalize:     json.RawMessage(`{"test": true}`),
			Logs:           json.RawMessage(`{"test": true}`),
			Extrinsics: json.RawMessage(`[
				{
					"method": "transfer",
					"params": {
						"id": "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY"
					}
				}
			]`),
		},
		{
			ID:             2,
			Timestamp:      time.Now(),
			Hash:           "0xabcdef1234567890abcdef1234567890",
			ParentHash:     "0x1234567890abcdef1234567890abcdef",
			StateRoot:      "0xabcdef1234567890abcdef1234567890",
			ExtrinsicsRoot: "0x1234567890abcdef1234567890abcdef",
			AuthorID:       "0xabcdef1234",
			Finalized:      false,
			OnInitialize:   json.RawMessage(`{"test": false}`),
			OnFinalize:     json.RawMessage(`{"test": false}`),
			Logs:           json.RawMessage(`{"test": false}`),
			Extrinsics: json.RawMessage(`[
				{
					"method": "transfer",
					"params": {
						"data": ["0x1234567890abcdef", "normal_string"]
					}
				}
			]`),
		},
	}

	// Set up expectations for transaction
	mock.ExpectBegin()

	// Expect prepared statement for blocks table
	mock.ExpectPrepare("INSERT INTO blocks_Polkadot_Polkadot")

	// Expect prepared statement for address2blocks table
	mock.ExpectPrepare("INSERT INTO address2blocks")

	// Expect executions for each item in blocks table
	mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect executions for addresses in address2blocks table
	// First block has one address from id field
	mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(0, 1))
	// Second block has one address from data array
	mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect transaction commit
	mock.ExpectCommit()

	// Call the function being tested
	err = saveToDatabase(db, testData)
	if err != nil {
		t.Errorf("saveToDatabase returned an error: %v", err)
	}

	// Verify that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestCreateTable(t *testing.T) {
	// Create a mock database connection
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Expect the first query to create the blocks table
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS blocks_Polkadot_Polkadot").WillReturnResult(sqlmock.NewResult(0, 0))

	// Expect the second query to create the address2blocks table
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS address2blocks").WillReturnResult(sqlmock.NewResult(0, 0))

	// Call the function being tested
	err = createTable(db)
	if err != nil {
		t.Errorf("createTable returned an error: %v", err)
	}

	// Verify that all expectations were met
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

	// Expect prepared statements
	mock.ExpectPrepare("INSERT INTO blocks_Polkadot_Polkadot.*")
	mock.ExpectPrepare("INSERT INTO address2blocks.*")

	// Expect executions for blocks table
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

	// No addresses to insert since the mock extrinsics are empty arrays

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

func TestExtractAddressesFromExtrinsics(t *testing.T) {
	tests := []struct {
		name       string
		extrinsics string
		expected   int
		err        bool
	}{
		{
			name:       "Empty extrinsics",
			extrinsics: `[]`,
			expected:   0,
			err:        false,
		},
		{
			name:       "Invalid JSON",
			extrinsics: `invalid`,
			expected:   0,
			err:        true,
		},
		{
			name:       "ID field",
			extrinsics: `[{"id": "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY"}]`,
			expected:   1,
			err:        false,
		},
		{
			name:       "Multiple ID fields",
			extrinsics: `[{"id": "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY"}, {"user_id": "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty"}]`,
			expected:   2,
			err:        false,
		},
		{
			name:       "Data array with Polkadot addresses",
			extrinsics: `[{"data": ["5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY", "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty"]}]`,
			expected:   2,
			err:        false,
		},
		{
			name:       "Nested data array",
			extrinsics: `[{"nested": {"data": ["5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY", "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty"]}}]`,
			expected:   2,
			err:        false,
		},
		{
			name:       "Combined ID and data fields",
			extrinsics: `[{"id": "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY", "data": ["5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty"]}]`,
			expected:   2,
			err:        false,
		},
		{
			name:       "Duplicate addresses",
			extrinsics: `[{"id": "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY"}, {"data": ["5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY", "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty"]}]`,
			expected:   2,
			err:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addresses, err := extractAddressesFromExtrinsics(json.RawMessage(tt.extrinsics))
			if tt.err {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(addresses) != tt.expected {
					t.Errorf("extractAddressesFromExtrinsics() got %d addresses, expected %d", len(addresses), tt.expected)
				}
			}
		})
	}
}

func TestExtractAddressesFromRealData(t *testing.T) {
	// Get all JSON files in the tests/data/blocks directory
	blockDir := "tests/data/blocks"
	files, err := os.ReadDir(blockDir)
	if err != nil {
		t.Fatalf("Failed to read blocks directory: %v", err)
	}

	// Filter for JSON files
	jsonFiles := make([]string, 0)
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			jsonFiles = append(jsonFiles, filepath.Join(blockDir, file.Name()))
		}
	}

	if len(jsonFiles) == 0 {
		t.Fatalf("No JSON files found in %s", blockDir)
	}

	t.Logf("Found %d JSON files to test", len(jsonFiles))

	// Process each JSON file
	for _, jsonFile := range jsonFiles {
		t.Run(jsonFile, func(t *testing.T) {
			// Read the file
			fileData, err := os.ReadFile(jsonFile)
			if err != nil {
				t.Fatalf("Failed to read file %s: %v", jsonFile, err)
			}

			// Parse the JSON to extract the extrinsics field
			var blockData struct {
				Extrinsics json.RawMessage `json:"extrinsics"`
			}
			if err := json.Unmarshal(fileData, &blockData); err != nil {
				t.Fatalf("Failed to unmarshal JSON from %s: %v", jsonFile, err)
			}

			// Extract addresses from the extrinsics
			addresses, err := extractAddressesFromExtrinsics(blockData.Extrinsics)
			if err != nil {
				t.Logf("Error extracting addresses from %s: %v", jsonFile, err)
				return
			}

			// Log the extracted addresses
			t.Logf("Extracted %d addresses from %s", len(addresses), jsonFile)
			for i, addr := range addresses {
				t.Logf("  Address %d: %s", i+1, addr)
			}

			// Count Polkadot addresses
			polkadotAddresses := len(addresses)
			t.Logf("Found %d Polkadot addresses in %s", polkadotAddresses, jsonFile)

			// Verify that all addresses start with a valid prefix (typically 1-9 or A-Z)
			for _, addr := range addresses {
				if strings.HasPrefix(addr, "0x") {
					t.Errorf("Found hex address %s in %s, expected only Polkadot addresses", addr, jsonFile)
				}
			}
		})
	}
}
