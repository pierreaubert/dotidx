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
