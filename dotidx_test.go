package dotidx

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

func TestFetchHeadBlock(t *testing.T) {
	// Create a test server with a handler that returns a mock head block
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/blocks/head" {
			t.Errorf("Expected request to /blocks/head, got %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Return a mock head block response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Create a mock block with ID 12345
		mockBlock := BlockData{
			ID:             12345,
			Timestamp:      time.Now(),
			Hash:           "0xabcdef1234567890",
			ParentHash:     "0x1234567890abcdef",
			StateRoot:      "0xabcdef1234567890",
			ExtrinsicsRoot: "0x1234567890abcdef",
			AuthorID:       "0xabcdef1234",
			Finalized:      true,
		}

		json.NewEncoder(w).Encode(mockBlock)
	}))
	defer server.Close()

	// Call the fetchHeadBlock function with the test server URL
	headBlock, err := fetchHeadBlock(server.URL)
	if err != nil {
		t.Fatalf("fetchHeadBlock returned an error: %v", err)
	}

	// Verify the returned head block ID
	expectedID := 12345
	if headBlock != expectedID {
		t.Errorf("Expected head block ID %d, got %d", expectedID, headBlock)
	}
}

func TestLiveMode(t *testing.T) {
	// Create a test server with handlers for both head block and block range requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/blocks/head" {
			// Return a mock head block response
			mockHeadBlock := BlockData{
				ID:             5,
				Timestamp:      time.Now(),
				Hash:           "0xabcdef1234567890",
				ParentHash:     "0x1234567890abcdef",
				StateRoot:      "0xabcdef1234567890",
				ExtrinsicsRoot: "0x1234567890abcdef",
				AuthorID:       "0xabcdef1234",
				Finalized:      true,
			}
			json.NewEncoder(w).Encode(mockHeadBlock)
			return
		}

		if r.URL.Path == "/blocks" {
			// Handle range parameter
			rangeParam := r.URL.Query().Get("range")
			if rangeParam == "" {
				// Handle single block request
				idStr := r.URL.Query().Get("id")
				id, err := strconv.Atoi(idStr)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				// Return a mock block for the requested ID
				mockBlock := BlockData{
					ID:             id,
					Timestamp:      time.Now(),
					Hash:           fmt.Sprintf("0xhash%d", id),
					ParentHash:     fmt.Sprintf("0xparent%d", id),
					StateRoot:      "0xstateroot",
					ExtrinsicsRoot: "0xextrinsicsroot",
					AuthorID:       "0xauthor",
					Finalized:      true,
				}
				json.NewEncoder(w).Encode(mockBlock)
				return
			}

			// Parse range parameter (format: start-end)
			parts := strings.Split(rangeParam, "-")
			if len(parts) != 2 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			startID, err := strconv.Atoi(parts[0])
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			endID, err := strconv.Atoi(parts[1])
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// Return mock blocks for the requested range
			var blocks []BlockData
			for id := startID; id <= endID; id++ {
				blocks = append(blocks, BlockData{
					ID:             id,
					Timestamp:      time.Now(),
					Hash:           fmt.Sprintf("0xhash%d", id),
					ParentHash:     fmt.Sprintf("0xparent%d", id),
					StateRoot:      "0xstateroot",
					ExtrinsicsRoot: "0xextrinsicsroot",
					AuthorID:       "0xauthor",
					Finalized:      true,
					Extrinsics:     json.RawMessage(`[]`),
				})
			}
			json.NewEncoder(w).Encode(blocks)
			return
		}

		// Default response for other paths
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create a test database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	// Set up expectations for existing blocks query
	mock.ExpectQuery("SELECT block_id FROM blocks_polkadot_chain WHERE block_id BETWEEN \\$1 AND \\$2").
		WithArgs(1, 5).
		WillReturnRows(sqlmock.NewRows([]string{"block_id"})) // No blocks exist yet

	// Expect transaction for batch insert
	mock.ExpectBegin()

	// Expect prepared statements
	mock.ExpectPrepare("INSERT INTO blocks_polkadot_chain.*")
	mock.ExpectPrepare("INSERT INTO address2blocks.*")

	// Expect executions for blocks table (5 blocks)
	for i := 0; i < 5; i++ {
		mock.ExpectExec("INSERT INTO blocks_polkadot_chain.*").
			WithArgs(
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}

	// Expect transaction commit
	mock.ExpectCommit()

	// Create a test config with live mode enabled
	config := Config{
		StartRange:   100, // These will be overridden in live mode
		EndRange:     200, // These will be overridden in live mode
		SidecarURL:   server.URL,
		PostgresURI:  "mock",
		BatchSize:    10,
		MaxWorkers:   2,
		FlushTimeout: 100 * time.Millisecond,
		Relaychain:   "polkadot",
		Chain:        "chain",
		Live:         true,
	}

	// Create a context with a short timeout for testing
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Call the function that would be called in live mode
	headBlock, err := fetchHeadBlock(config.SidecarURL)
	if err != nil {
		t.Fatalf("fetchHeadBlock returned an error: %v", err)
	}

	// Verify the head block ID
	if headBlock != 5 {
		t.Errorf("Expected head block ID 5, got %d", headBlock)
	}

	// Update the config with the head block
	config.StartRange = 1
	config.EndRange = headBlock

	// Run startWorkers to process existing blocks
	startWorkers(ctx, config, db)

	// Check expectations
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

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
		if r.URL.Path != "/blocks" || r.URL.Query().Get("id") != "1" {
			t.Errorf("Expected request to '/blocks?id=1', got '%s' with query params %v", r.URL.Path, r.URL.Query())
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		// Return a mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{
			"number": 1,
			"timestamp": "2023-01-01T00:00:00Z",
			"hash": "0x1234567890abcdef",
			"parentHash": "0xabcdef1234567890",
			"stateRoot": "0x1234567890abcdef1234567890abcdef",
			"extrinsicsRoot": "0xabcdef1234567890abcdef1234567890",
			"authorId": "0x1234567890",
			"finalized": true,
			"onInitialize": {},
			"onFinalize": {},
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

func TestFetchBlockRange(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the request URL matches the expected pattern for block range
		if r.URL.Path != "/blocks" {
			t.Errorf("Expected request to '/blocks', got '%s'", r.URL.Path)
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		// Check query parameters
		query := r.URL.Query()
		rangeParam := query.Get("range")

		if rangeParam != "100-105" {
			t.Errorf("Expected query parameter range=100-105, got range=%s", rangeParam)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Return a mock response with multiple blocks
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return an array of blocks
			fmt.Fprintln(w, `[
			{
				"number": 100,
				"timestamp": "2023-01-01T00:00:00Z",
				"hash": "0x1234567890abcdef1",
				"parentHash": "0xabcdef1234567890",
				"stateRoot": "0x1234567890abcdef1234567890abcdef",
				"extrinsicsRoot": "0xabcdef1234567890abcdef1234567890",
				"authorId": "0x1234567890",
				"finalized": true,
				"onInitialize": {},
				"onFinalize": {},
				"logs": [],
				"extrinsics": []
			},
			{
				"number": 101,
				"timestamp": "2023-01-01T00:01:00Z",
				"hash": "0x1234567890abcdef2",
				"parentHash": "0x1234567890abcdef1",
				"stateRoot": "0x1234567890abcdef1234567890abcdef",
				"extrinsicsRoot": "0xabcdef1234567890abcdef1234567890",
				"authorId": "0x1234567890",
				"finalized": true,
				"onInitialize": {},
				"onFinalize": {},
				"logs": [],
				"extrinsics": []
			},
			{
				"number": 102,
				"timestamp": "2023-01-01T00:02:00Z",
				"hash": "0x1234567890abcdef3",
				"parentHash": "0x1234567890abcdef2",
				"stateRoot": "0x1234567890abcdef1234567890abcdef",
				"extrinsicsRoot": "0xabcdef1234567890abcdef1234567890",
				"authorId": "0x1234567890",
				"finalized": true,
				"onInitialize": {},
				"onFinalize": {},
				"logs": [],
				"extrinsics": []
			}
		]`)
	}))
	defer server.Close()

	// Create a context
	ctx := context.Background()

	// Create an array of block IDs to fetch
	blockIDs := []int{100, 101, 102, 103, 104, 105}

	// Call the function to fetch blocks with the specified IDs
	blocks, err := fetchBlockRange(ctx, blockIDs, server.URL)
	if err != nil {
		t.Fatalf("fetchBlockRange returned an error: %v", err)
	}

	// Check that we got the expected number of blocks
	if len(blocks) != 3 {
		t.Fatalf("Expected 3 blocks, got %d", len(blocks))
	}

	// Check the first block
	if blocks[0].ID != 100 {
		t.Errorf("Expected first block ID=100, got %d", blocks[0].ID)
	}
	if blocks[0].Hash != "0x1234567890abcdef1" {
		t.Errorf("Expected first block Hash=0x1234567890abcdef1, got %s", blocks[0].Hash)
	}

	// Check the second block
	if blocks[1].ID != 101 {
		t.Errorf("Expected second block ID=101, got %d", blocks[1].ID)
	}
	if blocks[1].Hash != "0x1234567890abcdef2" {
		t.Errorf("Expected second block Hash=0x1234567890abcdef2, got %s", blocks[1].Hash)
	}

	// Check the third block
	if blocks[2].ID != 102 {
		t.Errorf("Expected third block ID=102, got %d", blocks[2].ID)
	}
	if blocks[2].Hash != "0x1234567890abcdef3" {
		t.Errorf("Expected third block Hash=0x1234567890abcdef3, got %s", blocks[2].Hash)
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
	mock.ExpectPrepare("INSERT INTO blocks_polkadot_chain")

	// Expect prepared statement for address2blocks table
	mock.ExpectPrepare("INSERT INTO address2blocks")

	// Expect executions for each item in blocks table
	mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect executions for addresses in address2blocks table
	// First block has one address from id field
	mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect transaction commit
	mock.ExpectCommit()

	// Create a minimal config for testing
	testConfig := Config{
		Relaychain: "polkadot",
		Chain:      "chain",
	}

	// Call the function being tested
	err = saveToDatabase(db, testData, testConfig)
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
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS blocks_polkadot_chain").WillReturnResult(sqlmock.NewResult(0, 0))

	// Expect the second query to create the address2blocks table
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS address2blocks_polkadot_chain").WillReturnResult(sqlmock.NewResult(0, 0))

	// Expect the third query to create the index on address column
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS address2blocks_polkadot_chain_address_idx").WillReturnResult(sqlmock.NewResult(0, 0))

	// Create a minimal config for testing
	testConfig := Config{
		Relaychain: "polkadot",
		Chain:      "chain",
	}

	// Call the function being tested
	err = createTable(db, testConfig)
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
		// Check if this is a request with an ID parameter
		if r.URL.Path == "/blocks" && r.URL.Query().Get("id") != "" {
			// Handle individual block request by ID parameter
			blockID := r.URL.Query().Get("id")

			// Return a mock response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{
				"id": %s,
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
			}`, blockID, blockID)
			return
		}

		// Check if this is a range request
		if r.URL.Path == "/blocks" {
			// Handle range request
			rangeParam := r.URL.Query().Get("range")
			if rangeParam == "" {
				t.Errorf("Missing range parameter in URL: %s", r.URL.String())
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}

			// Parse range parameter (format: start-end)
			parts := strings.Split(rangeParam, "-")
			if len(parts) != 2 {
				t.Errorf("Invalid range format: %s", rangeParam)
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}

			// Return a mock response with multiple blocks
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			// Start building the response array
			fmt.Fprint(w, "[")

			// Add block entries
			start, _ := strconv.Atoi(parts[0])
			end, _ := strconv.Atoi(parts[1])
			for i := start; i <= end; i++ {
				// Skip blocks that should already exist in the database
				if i == 2 || i == 4 {
					continue
				}

				if i > start {
					fmt.Fprint(w, ",")
				}

				fmt.Fprintf(w, `{
					"id": %d,
					"timestamp": "2023-01-01T00:00:00Z",
					"hash": "0x%d",
					"parenthash": "0xabcdef1234567890",
					"stateroot": "0x1234567890abcdef1234567890abcdef",
					"extrinsicsroot": "0xabcdef1234567890abcdef1234567890",
					"authorid": "0x1234567890",
					"finalized": true,
					"oninitialize": {},
					"onfinalize": {},
					"logs": [],
					"extrinsics": []
				}`, i, i)
			}

			// Close the array
			fmt.Fprint(w, "]")
			return
		}

		// Handle individual block requests by path
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
			"id": %s,
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
		}`, parts[2], parts[2]) // Use block ID for both id and hash
	}))
	defer server.Close()

	// Create a test database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	// Set up expectations
	mock.ExpectQuery("SELECT block_id FROM blocks_polkadot_chain WHERE block_id BETWEEN \\$1 AND \\$2").
		WithArgs(1, 5).
		WillReturnRows(sqlmock.NewRows([]string{"block_id"}).AddRow(2).AddRow(4)) // Blocks 2 and 4 already exist

	// Expect transaction for batch insert
	mock.ExpectBegin()

	// Expect prepared statements
	mock.ExpectPrepare("INSERT INTO blocks_polkadot_chain.*")
	mock.ExpectPrepare("INSERT INTO address2blocks.*")

	// Expect executions for blocks table (one for each non-existing block: 1, 3, 5)
	// Block 1
	mock.ExpectExec("INSERT INTO blocks_polkadot_chain.*").
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Block 3
	mock.ExpectExec("INSERT INTO blocks_polkadot_chain.*").
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Block 5
	mock.ExpectExec("INSERT INTO blocks_polkadot_chain.*").
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
		Relaychain:   "polkadot",
		Chain:        "chain",
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

func TestBatchProcessing(t *testing.T) {
	// Create a test database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	// Set up expectations for existing blocks query
	mock.ExpectQuery("SELECT block_id FROM blocks_polkadot_chain WHERE block_id BETWEEN \\$1 AND \\$2").
		WithArgs(1, 10).
		WillReturnRows(sqlmock.NewRows([]string{"block_id"}).
			AddRow(2).AddRow(5).AddRow(6)) // Blocks 2, 5, and 6 exist

	// Set up expectations for database transactions
	mock.ExpectBegin()

	// Prepare statements expectations
	mock.ExpectPrepare("INSERT INTO blocks_polkadot_chain \\(.*\\) VALUES \\(.*\\)")
	mock.ExpectPrepare("INSERT INTO address2blocks_polkadot_chain \\(.*\\) VALUES \\(.*\\)")

	// Expect executions for blocks 1, 3, 4 (continuous range)
	mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(1, 1))

	// Expect execution for blocks 7, 8, 9, 10 (continuous range)
	mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(1, 1))

	// Expect commit
	mock.ExpectCommit()

	// Create a test server for sidecar API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a request for a range of blocks or a single block
		if strings.Contains(r.URL.Path, "/blocks") && r.URL.Query().Get("range") != "" {
			// This is a batch request
			w.Header().Set("Content-Type", "application/json")

			// Return different responses based on the range
			if r.URL.Query().Get("range") == "1-4" {
				// Return blocks 1, 3, 4 (skipping 2 which exists)
				fmt.Fprintln(w, `[
				{
					"id": 1,
					"timestamp": "2023-01-01T00:00:00Z",
					"hash": "0x1234567890abcdef1",
					"parentHash": "0x0000000000000000",
					"stateRoot": "0x1234567890abcdef1234567890abcdef",
					"extrinsicsRoot": "0xabcdef1234567890abcdef1234567890",
					"authorId": "0x1234567890",
					"finalized": true,
					"onInitialize": {},
					"onFinalize": {},
					"logs": [],
					"extrinsics": []
				},
				{
					"id": 3,
					"timestamp": "2023-01-01T00:00:00Z",
					"hash": "0x1234567890abcdef3",
					"parentHash": "0x1234567890abcdef2",
					"stateRoot": "0x1234567890abcdef1234567890abcdef",
					"extrinsicsRoot": "0xabcdef1234567890abcdef1234567890",
					"authorId": "0x1234567890",
					"finalized": true,
					"onInitialize": {},
					"onFinalize": {},
					"logs": [],
					"extrinsics": []
				},
				{
					"id": 4,
					"timestamp": "2023-01-01T00:00:00Z",
					"hash": "0x1234567890abcdef4",
					"parentHash": "0x1234567890abcdef3",
					"stateRoot": "0x1234567890abcdef1234567890abcdef",
					"extrinsicsRoot": "0xabcdef1234567890abcdef1234567890",
					"authorId": "0x1234567890",
					"finalized": true,
					"onInitialize": {},
					"onFinalize": {},
					"logs": [],
					"extrinsics": []
				}
			]`)
			} else if r.URL.Query().Get("range") == "7-10" {
				// Return blocks 7, 8, 9, 10
				fmt.Fprintln(w, `[
				{
					"id": 7,
					"timestamp": "2023-01-01T00:00:00Z",
					"hash": "0x1234567890abcdef7",
					"parentHash": "0x1234567890abcdef6",
					"stateRoot": "0x1234567890abcdef1234567890abcdef",
					"extrinsicsRoot": "0xabcdef1234567890abcdef1234567890",
					"authorId": "0x1234567890",
					"finalized": true,
					"onInitialize": {},
					"onFinalize": {},
					"logs": [],
					"extrinsics": []
				},
				{
					"id": 8,
					"timestamp": "2023-01-01T00:00:00Z",
					"hash": "0x1234567890abcdef8",
					"parentHash": "0x1234567890abcdef7",
					"stateRoot": "0x1234567890abcdef1234567890abcdef",
					"extrinsicsRoot": "0xabcdef1234567890abcdef1234567890",
					"authorId": "0x1234567890",
					"finalized": true,
					"onInitialize": {},
					"onFinalize": {},
					"logs": [],
					"extrinsics": []
				},
				{
					"id": 9,
					"timestamp": "2023-01-01T00:00:00Z",
					"hash": "0x1234567890abcdef9",
					"parentHash": "0x1234567890abcdef8",
					"stateRoot": "0x1234567890abcdef1234567890abcdef",
					"extrinsicsRoot": "0xabcdef1234567890abcdef1234567890",
					"authorId": "0x1234567890",
					"finalized": true,
					"onInitialize": {},
					"onFinalize": {},
					"logs": [],
					"extrinsics": []
				},
				{
					"id": 10,
					"timestamp": "2023-01-01T00:00:00Z",
					"hash": "0x1234567890abcdef10",
					"parentHash": "0x1234567890abcdef9",
					"stateRoot": "0x1234567890abcdef1234567890abcdef",
					"extrinsicsRoot": "0xabcdef1234567890abcdef1234567890",
					"authorId": "0x1234567890",
					"finalized": true,
					"onInitialize": {},
					"onFinalize": {},
					"logs": [],
					"extrinsics": []
				}
			]`)
			}
		} else if strings.Contains(r.URL.Path, "/block") {
			// This is a single block request
			w.Header().Set("Content-Type", "application/json")

			// Extract block ID from the URL
			parts := strings.Split(r.URL.Path, "/")
			blockIDStr := parts[len(parts)-1]
			blockID, _ := strconv.Atoi(blockIDStr)

			// Return the requested block
			fmt.Fprintf(w, `{
				"id": %d,
				"timestamp": "2023-01-01T00:00:00Z",
				"hash": "0x1234567890abcdef%d",
				"parentHash": "0x1234567890abcdef%d",
				"stateRoot": "0x1234567890abcdef1234567890abcdef",
				"extrinsicsRoot": "0xabcdef1234567890abcdef1234567890",
				"authorId": "0x1234567890",
				"finalized": true,
				"onInitialize": {},
				"onFinalize": {},
				"logs": [],
				"extrinsics": []
			}`, blockID, blockID, blockID-1)
		}
	}))
	defer server.Close()

	// Create a test config
	config := Config{
		StartRange:   1,
		EndRange:     10,
		SidecarURL:   server.URL,
		PostgresURI:  "mock",
		BatchSize:    3, // Small batch size to test batch processing
		MaxWorkers:   4,
		FlushTimeout: 100 * time.Millisecond,
		Relaychain:   "polkadot",
		Chain:        "chain",
		Live:         false,
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
