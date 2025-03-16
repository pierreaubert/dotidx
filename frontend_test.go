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
)

func TestHandleAddressToBlocks(t *testing.T) {
	// Create a new mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Create test config
	config := Config{
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
				mock.ExpectQuery(`FROM chain\.blocks.*JOIN chain\.address2blocks`).WithArgs(
					"5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty",
				).WillReturnRows(mockRows)
			},
			validateResult: func(t *testing.T, resp *httptest.ResponseRecorder) {
				if resp.Code != http.StatusOK {
					t.Errorf("Expected status %d, got %d", http.StatusOK, resp.Code)
				}

				// Decode response body
				var response BlocksByAddressResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to decode response body: %v", err)
				}

				// Validate response
				if response.Address != "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty" {
					t.Errorf("Expected address %s, got %s", "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty", response.Address)
				}

				if len(response.Blocks) != 1 {
					t.Errorf("Expected 1 block, got %d", len(response.Blocks))
				}

				if response.Blocks[0].ID != "12345" {
					t.Errorf("Expected block ID 12345, got %s", response.Blocks[0].ID)
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
				mock.ExpectQuery(`FROM chain\.blocks.*JOIN chain\.address2blocks`).WithArgs(
					"5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty",
				).WillReturnError(fmt.Errorf("database error"))
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

func TestHandleCompletionRate(t *testing.T) {
	// Create a new mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Create test config
	config := Config{
		Relaychain: "Polkadot",
		Chain:      "Polkadot",
	}

	// Create frontend instance
	frontend := NewFrontend(db, config, ":8080")

	// Test cases
	testCases := []struct {
		name           string
		method         string
		expectStatus   int
		mockQuery      func()
		validateResult func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:         "Valid request with results",
			method:       http.MethodGet,
			expectStatus: http.StatusOK,
			mockQuery: func() {
				// Create expected rows
				columns := []string{"percentcompletion", "headID"}

				// Create mock rows with completion rate of 85% and head ID of 1000
				mockRows := sqlmock.NewRows(columns).AddRow(85, 1000)

				// Expect query execution for completion rate
				mock.ExpectQuery(`SELECT.+count\(distinct block_id\).+FROM chain\.blocks_polkadot_polkadot`).WillReturnRows(mockRows)
			},
			validateResult: func(t *testing.T, resp *httptest.ResponseRecorder) {
				if resp.Code != http.StatusOK {
					t.Errorf("Expected status %d, got %d", http.StatusOK, resp.Code)
				}

				// Decode response body
				var response CompletionRateResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to decode response body: %v", err)
				}

				// Validate response
				if response.PercentCompletion != 85 {
					t.Errorf("Expected percent completion 85, got %d", response.PercentCompletion)
				}

				if response.HeadID != 1000 {
					t.Errorf("Expected head ID 1000, got %d", response.HeadID)
				}
			},
		},
		{
			name:         "Invalid request method",
			method:       http.MethodPost,
			expectStatus: http.StatusMethodNotAllowed,
			mockQuery:    func() {},
			validateResult: func(t *testing.T, resp *httptest.ResponseRecorder) {
				if resp.Code != http.StatusMethodNotAllowed {
					t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, resp.Code)
				}
			},
		},
		{
			name:         "Database error",
			method:       http.MethodGet,
			expectStatus: http.StatusInternalServerError,
			mockQuery: func() {
				mock.ExpectQuery(`SELECT.+count\(distinct block_id\).+FROM chain\.blocks_polkadot_polkadot`).WillReturnError(fmt.Errorf("database error"))
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
			req := httptest.NewRequest(tc.method, "/stats/completatiorate", nil)

			// Create response recorder
			rec := httptest.NewRecorder()

			// Handle request
			frontend.handleCompletionRate(rec, req)

			// Validate result
			tc.validateResult(t, rec)

			// Verify all expectations were met
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("Unfulfilled expectations: %s", err)
			}
		})
	}
}

func TestHandleStatsPerMonth(t *testing.T) {
	// Create a new mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Create test config
	config := Config{
		Relaychain: "Polkadot",
		Chain:      "Polkadot",
	}

	// Create frontend instance
	frontend := NewFrontend(db, config, ":8080")

	// Test cases
	testCases := []struct {
		name           string
		method         string
		expectStatus   int
		mockQuery      func()
		validateResult func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:         "Valid request with results",
			method:       http.MethodGet,
			expectStatus: http.StatusOK,
			mockQuery: func() {
				// Reset cache before test
				frontend.cacheMutex.Lock()
				frontend.monthlyStatsCache = nil
				frontend.monthlyStatsCacheExp = time.Now().Add(-1 * time.Hour)
				frontend.cacheMutex.Unlock()
				
				// Create expected rows
				columns := []string{"date", "count", "min", "max"}

				// Create mock rows
				mockRows := sqlmock.NewRows(columns).AddRow(
					time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), // date
					100,                                       // count
					1000,                                      // min_block_id
					1100,                                      // max_block_id
				).AddRow(
					time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC), // date
					200,                                       // count
					1101,                                      // min_block_id
					1300,                                      // max_block_id
				)

				// Expect query execution
				mock.ExpectQuery(`SELECT.*date_trunc\('month',created_at\)::date as date`).WillReturnRows(mockRows)
			},
			validateResult: func(t *testing.T, resp *httptest.ResponseRecorder) {
				if resp.Code != http.StatusOK {
					t.Errorf("Expected status %d, got %d", http.StatusOK, resp.Code)
				}

				// Decode response body
				var response MonthlyStatsResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to decode response body: %v", err)
				}

				// Validate response
				if len(response.Data) != 2 {
					t.Errorf("Expected 2 months of data, got %d", len(response.Data))
				}

				if response.Data[0].Date != "2023-01" {
					t.Errorf("Expected first date to be 2023-01, got %s", response.Data[0].Date)
				}

				if response.Data[0].Count != 100 || response.Data[1].Count != 200 {
					t.Errorf("Count values incorrect")
				}

				if response.Data[0].MinBlock != 1000 || response.Data[1].MinBlock != 1101 {
					t.Errorf("MinBlock values incorrect")
				}

				if response.Data[0].MaxBlock != 1100 || response.Data[1].MaxBlock != 1300 {
					t.Errorf("MaxBlock values incorrect")
				}
			},
		},
		{
			name:         "Invalid request method",
			method:       http.MethodPost,
			expectStatus: http.StatusMethodNotAllowed,
			mockQuery:    func() {},
			validateResult: func(t *testing.T, resp *httptest.ResponseRecorder) {
				if resp.Code != http.StatusMethodNotAllowed {
					t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, resp.Code)
				}
			},
		},
		{
			name:         "Database error",
			method:       http.MethodGet,
			expectStatus: http.StatusInternalServerError,
			mockQuery: func() {
				// Reset cache before test
				frontend.cacheMutex.Lock()
				frontend.monthlyStatsCache = nil
				frontend.monthlyStatsCacheExp = time.Now().Add(-1 * time.Hour)
				frontend.cacheMutex.Unlock()
				
				mock.ExpectQuery(`SELECT.*date_trunc\('month',created_at\)::date as date`).WillReturnError(fmt.Errorf("database error"))
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
			req := httptest.NewRequest(tc.method, "/stats/per_month", nil)

			// Create response recorder
			rec := httptest.NewRecorder()

			// Handle request
			frontend.handleStatsPerMonth(rec, req)

			// Validate result
			tc.validateResult(t, rec)

			// Verify all expectations were met
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("Unfulfilled expectations: %s", err)
			}
		})
	}
}

func TestMonthlyStatsCaching(t *testing.T) {
	// Create a new mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Create test config
	config := Config{
		Relaychain: "Polkadot",
		Chain:      "Polkadot",
	}

	// Create frontend instance
	frontend := NewFrontend(db, config, ":8080")

	// Create expected rows
	columns := []string{"date", "count", "min", "max"}

	// Create mock rows that will be returned on first query
	mockRows := sqlmock.NewRows(columns).AddRow(
		time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), // date
		100,                                       // count
		1000,                                      // min_block_id
		1100,                                      // max_block_id
	).AddRow(
		time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC), // date
		200,                                       // count
		1101,                                      // min_block_id
		1300,                                      // max_block_id
	)

	// First query should execute the database query
	mock.ExpectQuery(`SELECT.*date_trunc\('month',created_at\)::date as date`).WillReturnRows(mockRows)

	// First request (should query the database)
	stats1, err := frontend.getCachedMonthlyStats()
	if err != nil {
		t.Fatalf("Error getting monthly stats: %v", err)
	}

	// Verify results from first query
	if len(stats1) != 2 {
		t.Errorf("Expected 2 months of data, got %d", len(stats1))
	}

	if stats1[0].Date != "2023-01" {
		t.Errorf("Expected first date to be 2023-01, got %s", stats1[0].Date)
	}

	// Cache should be updated now
	if !frontend.monthlyStatsCacheExp.After(time.Now()) {
		t.Errorf("Cache expiration should be in the future")
	}

	// The database should only be queried once during these tests
	// Make second request (should use cache)
	stats2, err := frontend.getCachedMonthlyStats()
	if err != nil {
		t.Fatalf("Error getting cached monthly stats: %v", err)
	}

	// Verify cached results match
	if len(stats2) != len(stats1) {
		t.Errorf("Cached result length doesn't match: %d vs %d", len(stats2), len(stats1))
	}

	if stats2[0].Date != stats1[0].Date || stats2[1].Date != stats1[1].Date {
		t.Errorf("Cached dates don't match original results")
	}

	if stats2[0].Count != stats1[0].Count || stats2[1].Count != stats1[1].Count {
		t.Errorf("Cached counts don't match original results")
	}

	// Verify all expectations were met (database should only be queried once)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}

	// Test cache expiration
	// Force expire the cache
	frontend.cacheMutex.Lock()
	frontend.monthlyStatsCacheExp = time.Now().Add(-1 * time.Minute) // Expired 1 minute ago
	frontend.cacheMutex.Unlock()

	// Create new mock rows for the new query after cache expiration
	newMockRows := sqlmock.NewRows(columns).AddRow(
		time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), // same first month
		100,                                       // count
		1000,                                      // min_block_id
		1100,                                      // max_block_id
	).AddRow(
		time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC), // different second month
		300,                                       // count
		1301,                                      // min_block_id
		1500,                                      // max_block_id
	)

	// Expect another query after cache expiration
	mock.ExpectQuery(`SELECT.*date_trunc\('month',created_at\)::date as date`).WillReturnRows(newMockRows)

	// Request after cache expiration (should query database again)
	stats3, err := frontend.getCachedMonthlyStats()
	if err != nil {
		t.Fatalf("Error getting monthly stats after cache expiration: %v", err)
	}

	// Verify we got the new data
	if len(stats3) != 2 {
		t.Errorf("Expected 2 months of data after cache expiration, got %d", len(stats3))
	}

	if stats3[1].Date != "2023-03" {
		t.Errorf("Expected second date to be 2023-03 after cache expiration, got %s", stats3[1].Date)
	}

	if stats3[1].Count != 300 {
		t.Errorf("Expected count of 300 for second month after cache expiration, got %d", stats3[1].Count)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestIsValidAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    bool
	}{
		{
			name:    "Valid Polkadot address",
			address: "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty",
			want:    true,
		},
		{
			name:    "Valid address with 1 prefix",
			address: "16fAYQeYwBhWrJGSS8UXMNUWvUQf38VcvCaXxUPwMBUCCsQ1",
			want:    true,
		},
		{
			name:    "Too short address",
			address: "5FHne",
			want:    false,
		},
		{
			name:    "Too long address",
			address: "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694tyaaaaaaaaaaaa",
			want:    false,
		},
		{
			name:    "Invalid prefix",
			address: "XYZ123W46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty",
			want:    false,
		},
		{
			name:    "Empty address",
			address: "",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidAddress(tt.address); got != tt.want {
				t.Errorf("isValidAddress() = %v, want %v", got, tt.want)
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
	config := Config{
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
