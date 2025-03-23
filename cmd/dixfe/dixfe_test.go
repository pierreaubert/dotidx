package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/pierreaubert/dotidx"
)

func TestHandleAddressToBlocks(t *testing.T) {
	// Create a new mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Create test config
	config := dotidx.Config{
		Relaychain: "Polkadot",
		Chain:      "Polkadot",
	}

	// Create frontend instance
	frontend := NewFrontend(db, config, ":8080")

	// Test cases
	testCases := []struct {
		name           string
		address        string
		method         string
		expectStatus   int
		expectRows     bool
		mockQuery      func()
		validateResult func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:         "Valid address with results",
			address:      "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty",
			method:       http.MethodGet,
			expectStatus: http.StatusOK,
			expectRows:   true,
			mockQuery: func() {
				// Create expected rows
				columns := []string{
					"block_id", "created_at", "hash", "parent_hash", "state_root",
					"extrinsics_root", "author_id", "finalized", "on_initialize",
					"on_finalize", "logs", "extrinsics",
				}

				// Create mock rows
				mockRows := sqlmock.NewRows(columns).AddRow(
					12345,      // block_id
					time.Now(), // created_at
					"0xabc123", // hash
					"0xdef456", // parent_hash
					"0xfff789", // state_root
					"0x123abc", // extrinsics_root
					"5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty", // author_id
					true,         // finalized
					[]byte("[]"), // on_initialize
					[]byte("[]"), // on_finalize
					[]byte("[]"), // logs
					[]byte(`[{"id":"5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty"}]`), // extrinsics
				)

				// Expect query execution with join between address2blocks and blocks tables
				mock.ExpectQuery(`SELECT.*FROM chain\.blocks.*JOIN chain\.address2blocks.*WHERE a\.address = '5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty'.*`).WillReturnRows(mockRows)
			},
			validateResult: func(t *testing.T, resp *httptest.ResponseRecorder) {
				if resp.Code != http.StatusOK {
					t.Errorf("Expected status %d, got %d", http.StatusOK, resp.Code)
				}

				// Decode response body - now expecting an array of blocks directly, not a BlocksResponse
				var blocks []dotidx.BlockData
				if err := json.Unmarshal(resp.Body.Bytes(), &blocks); err != nil {
					t.Fatalf("Failed to decode response body: %v", err)
				}

				// Validate response
				if len(blocks) == 0 || blocks[0].ID != "12345" {
					t.Errorf("Expected block ID 12345, got %v", blocks)
				}
			},
		},
		{
			name:         "Invalid request method",
			address:      "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty",
			method:       http.MethodPost,
			expectStatus: http.StatusMethodNotAllowed,
			expectRows:   false,
			mockQuery:    func() {},
			validateResult: func(t *testing.T, resp *httptest.ResponseRecorder) {
				if resp.Code != http.StatusMethodNotAllowed {
					t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, resp.Code)
				}
			},
		},
		{
			name:         "Missing address parameter",
			address:      "",
			method:       http.MethodGet,
			expectStatus: http.StatusBadRequest,
			expectRows:   false,
			mockQuery:    func() {},
			validateResult: func(t *testing.T, resp *httptest.ResponseRecorder) {
				if resp.Code != http.StatusBadRequest {
					t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.Code)
				}
			},
		},
		{
			name:         "Invalid address format",
			address:      "invalid-address",
			method:       http.MethodGet,
			expectStatus: http.StatusBadRequest,
			expectRows:   false,
			mockQuery:    func() {},
			validateResult: func(t *testing.T, resp *httptest.ResponseRecorder) {
				if resp.Code != http.StatusBadRequest {
					t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.Code)
				}
			},
		},
		{
			name:         "Database error",
			address:      "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty",
			method:       http.MethodGet,
			expectStatus: http.StatusInternalServerError,
			expectRows:   false,
			mockQuery: func() {
				mock.ExpectQuery(`SELECT.*FROM chain\.blocks.*JOIN chain\.address2blocks.*WHERE a\.address = '5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty'.*`).WillReturnError(fmt.Errorf("database error"))
			},
			validateResult: func(t *testing.T, resp *httptest.ResponseRecorder) {
				if resp.Code != http.StatusInternalServerError {
					t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, resp.Code)
				}
			},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up mock query expectations
			tc.mockQuery()

			// Create request
			url := "/address2blocks"
			if tc.address != "" {
				url = fmt.Sprintf("/address2blocks?address=%s", tc.address)
			}
			req := httptest.NewRequest(tc.method, url, nil)

			// Create response recorder
			rec := httptest.NewRecorder()

			// Handle request
			frontend.handleAddressToBlocks(rec, req)

			// Validate result
			tc.validateResult(t, rec)

			// Verify all expectations were met
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("Unfulfilled expectations: %s", err)
			}
		})
	}
}

func TestFrontendStart(t *testing.T) {
	// Create a new mock database
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Create test config
	config := dotidx.Config{
		Relaychain: "Polkadot",
		Chain:      "Polkadot",
	}

	// Create frontend instance with a random port (to avoid conflicts)
	frontend := NewFrontend(db, config, ":0")

	// Create context with cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Start frontend in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- frontend.Start(ctx.Done())
	}()

	// Wait a short time for server to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	// Wait for shutdown to complete with timeout
	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("Frontend.Start returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("Timeout waiting for server to shut down")
	}
}
