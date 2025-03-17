package main

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

func TestSaveToDatabase(t *testing.T) {
	// Create a mock database connection
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Create test data with valid Polkadot addresses that will be found
	testData := []BlockData{
		{
			ID:             "1",
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
			Extrinsics:     json.RawMessage(`[{"method": "timestamp.set","now": 1234567890,"signer_id": "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY"}]`),
		},
		{
			ID:             "2",
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
			Extrinsics:     json.RawMessage(`[{"method": "timestamp.set","now": 1234567890,"account_id": "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty"}]`),
		},
	}

	// Set up expectations for transaction
	mock.ExpectBegin()

	// For first item: first blocks table insert with correct column names
	mock.ExpectExec("^INSERT INTO chain\\.blocks_polkadot_chain \\(block_id, created_at, hash, parent_hash, state_root, extrinsics_root, author_id, finalized, on_initialize, on_finalize, logs, extrinsics\\) VALUES \\(.*\\) ON CONFLICT.*$").WillReturnResult(sqlmock.NewResult(0, 1))

	// Then address2blocks table
	mock.ExpectExec("^INSERT INTO chain\\.address2blocks_polkadot_chain \\(address, block_id\\) VALUES \\(\\$1, \\$2\\) ON CONFLICT \\(address, block_id\\) DO NOTHING$").WithArgs("5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY", "1").WillReturnResult(sqlmock.NewResult(0, 1))

	// For second item: first blocks table with correct column names
	mock.ExpectExec("^INSERT INTO chain\\.blocks_polkadot_chain \\(block_id, created_at, hash, parent_hash, state_root, extrinsics_root, author_id, finalized, on_initialize, on_finalize, logs, extrinsics\\) VALUES \\(.*\\) ON CONFLICT.*$").WillReturnResult(sqlmock.NewResult(0, 1))

	// Then address2blocks table
	mock.ExpectExec("^INSERT INTO chain\\.address2blocks_polkadot_chain \\(address, block_id\\) VALUES \\(\\$1, \\$2\\) ON CONFLICT \\(address, block_id\\) DO NOTHING$").WithArgs("5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty", "2").WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect transaction commit
	mock.ExpectCommit()

	// Create a minimal config for testing with batch size equal to our test data length
	// to ensure immediate flushing
	testConfig := Config{
		Relaychain:   "polkadot",
		Chain:        "chain",
		BatchSize:    2,                // Set batch size equal to our test data size to trigger immediate flush
		FlushTimeout: 10 * time.Second, // Long timeout to ensure size triggers flush, not time
	}

	database := NewSQLDatabase(db, context.Background())

	// Create a wait group to wait for async processing
	var wg sync.WaitGroup
	wg.Add(1)

	// Use a channel to signal when we should check results
	done := make(chan struct{})
	go func() {
		defer wg.Done()
		time.Sleep(300 * time.Millisecond) // Give time for batch processing
		done <- struct{}{}
	}()

	// Save data to database (should trigger immediate batch flush due to matching batch size)
	err = database.Save(testData, testConfig)
	assert.NoError(t, err, "Should not error when saving data")

	// Wait for batch processing to complete
	<-done
	wg.Wait()

	// Verify all expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err, "All expectations should be met")
}

func TestDatabasePoolConfig(t *testing.T) {
	// Test the default connection pool config
	defaultConfig := DefaultDBPoolConfig()

	assert.Equal(t, 25, defaultConfig.MaxOpenConns, "Default max open connections should be 25")
	assert.Equal(t, 5, defaultConfig.MaxIdleConns, "Default max idle connections should be 5")
	assert.Equal(t, 5*time.Minute, defaultConfig.ConnMaxLifetime, "Default connection max lifetime should be 5 minutes")
	assert.Equal(t, 1*time.Minute, defaultConfig.ConnMaxIdleTime, "Default connection max idle time should be 1 minute")
}

func TestNewSQLDatabaseWithPool(t *testing.T) {
	// Create a mock database connection
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Create a custom pool configuration
	customConfig := DBPoolConfig{
		MaxOpenConns:    50,
		MaxIdleConns:    10,
		ConnMaxLifetime: 10 * time.Minute,
		ConnMaxIdleTime: 2 * time.Minute,
	}

	// Create database with custom pool configuration
	database := NewSQLDatabaseWithPool(db, context.Background(), customConfig)

	// Verify that the pool configuration was stored correctly
	assert.Equal(t, customConfig.MaxOpenConns, database.poolCfg.MaxOpenConns, "MaxOpenConns should match")
	assert.Equal(t, customConfig.MaxIdleConns, database.poolCfg.MaxIdleConns, "MaxIdleConns should match")
	assert.Equal(t, customConfig.ConnMaxLifetime, database.poolCfg.ConnMaxLifetime, "ConnMaxLifetime should match")
	assert.Equal(t, customConfig.ConnMaxIdleTime, database.poolCfg.ConnMaxIdleTime, "ConnMaxIdleTime should match")

	// Note: We can't directly test the SetMaxOpenConns, SetMaxIdleConns, etc. calls
	// since sqlmock doesn't provide a way to verify those were called.
	// In a real-world scenario, we would test this with an actual database.
}

func TestDatabaseClose(t *testing.T) {
	// Create a mock database connection
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}

	// Create database instance
	database := NewSQLDatabase(db, context.Background())

	// Mock Close expectation
	mock.ExpectClose()

	// Call Close method
	err = database.Close()
	assert.NoError(t, err, "Close should not return an error")

	// Verify expectations
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// func TestCreateTable(t *testing.T) {
// 	// Create a mock database connection
// 	db, mock, err := sqlmock.New()
// 	if err != nil {
// 		t.Fatalf("Error creating mock database: %v", err)
// 	}
// 	defer db.Close()

// 	// Create a minimal config for testing
// 	testConfig := Config{
// 		Relaychain: "polkadot",
// 		Chain:      "chain",
// 	}

// 	// Verify that all expectations were met
// 	// Expect the first query to create the blocks table
// 	mock.ExpectExec("CREATE TABLE IF NOT EXISTS public.blocks_polkadot_chain").WillReturnResult(sqlmock.NewResult(0, 0))

// 	// Expect the second query to create the address2blocks table
// 	mock.ExpectExec("CREATE TABLE IF NOT EXISTS public.address2blocks_polkadot_chain").WillReturnResult(sqlmock.NewResult(0, 0))

// 	// Expect the third query to create the index on address column
// 	mock.ExpectExec("CREATE INDEX IF NOT EXISTS public.address2blocks_polkadot_chain_address_idx").WillReturnResult(sqlmock.NewResult(0, 0))

// 	database := NewSQLDatabase(db, context.Background())

// 	// Call the function being tested
// 	err = database.CreateTable(testConfig)
// 	if err != nil {
// 		t.Errorf("createTable returned an error: %v", err)
// 	}

// 	if err := mock.ExpectationsWereMet(); err != nil {
// 		t.Errorf("Unfulfilled expectations: %v", err)
// 	}
// }
