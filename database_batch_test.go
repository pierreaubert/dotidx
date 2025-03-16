package main

import (
	"encoding/json"
	"testing"
	"time"
	"sync"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestAsynchronousBatchProcessing(t *testing.T) {
	// Create a mock database connection
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Create test data
	testData := []BlockData{
		{
			ID:             "1",
			Timestamp:      time.Now(),
			Hash:           "hash1",
			ParentHash:     "parenthash1",
			StateRoot:      "stateroot1",
			ExtrinsicsRoot: "extrinsicsroot1",
			AuthorID:       "author1",
			Finalized:      true,
			OnInitialize:   json.RawMessage(`{"test": true}`),
			OnFinalize:     json.RawMessage(`{"test": true}`),
			Logs:           json.RawMessage(`{"test": true}`),
			Extrinsics: json.RawMessage(`[
				{
					"method": "timestamp.set",
					"now": 1234567890,
					"signer_id": "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY"
				}
			]`),
		},
		{
			ID:             "2",
			Timestamp:      time.Now(),
			Hash:           "hash2",
			ParentHash:     "parenthash2",
			StateRoot:      "stateroot2",
			ExtrinsicsRoot: "extrinsicsroot2",
			AuthorID:       "author2",
			Finalized:      true,
			OnInitialize:   json.RawMessage(`{"test": true}`),
			OnFinalize:     json.RawMessage(`{"test": true}`),
			Logs:           json.RawMessage(`{"test": true}`),
			Extrinsics: json.RawMessage(`[
				{
					"method": "timestamp.set",
					"now": 1234567890,
					"account_id": "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty"
				}
			]`),
		},
		{
			ID:             "3",
			Timestamp:      time.Now(),
			Hash:           "hash3",
			ParentHash:     "parenthash3",
			StateRoot:      "stateroot3",
			ExtrinsicsRoot: "extrinsicsroot3",
			AuthorID:       "author3",
			Finalized:      true,
			OnInitialize:   json.RawMessage(`{"test": true}`),
			OnFinalize:     json.RawMessage(`{"test": true}`),
			Logs:           json.RawMessage(`{"test": true}`),
			Extrinsics: json.RawMessage(`[
				{
					"method": "timestamp.set",
					"now": 1234567890,
					"validator_id": "5DAAnrj7VHTznn2AWBemMuyBwZWs6FNFjdyVXUeYum3PTXFy"
				}
			]`),
		},
	}

	// Create config with specified batch size to control number of items per batch
	testConfig := Config{
		Relaychain:   "test_relay",
		Chain:        "testchain",
		BatchSize:    2, // Set batch size to 2 to split our 3 items into two batches
		FlushTimeout: 10 * time.Second,
	}

	// Set up expectations for batches
	mock.ExpectBegin()
	
	// First batch (items 0 and 1)
	// For item 0: first blocks table, then address2blocks table
	mock.ExpectExec("^INSERT INTO chain\\.blocks_test_relay_testchain \\(block_id, created_at, hash, parent_hash, state_root, extrinsics_root, author_id, finalized, on_initialize, on_finalize, logs, extrinsics\\) VALUES \\(.*\\) ON CONFLICT.*$").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("^INSERT INTO chain\\.address2blocks_test_relay_testchain \\(address, block_id\\) VALUES \\(\\$1, \\$2\\) ON CONFLICT \\(address, block_id\\) DO NOTHING$").WithArgs("5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY", "1").WillReturnResult(sqlmock.NewResult(0, 1))
	
	// For item 1: first blocks table, then address2blocks table
	mock.ExpectExec("^INSERT INTO chain\\.blocks_test_relay_testchain \\(block_id, created_at, hash, parent_hash, state_root, extrinsics_root, author_id, finalized, on_initialize, on_finalize, logs, extrinsics\\) VALUES \\(.*\\) ON CONFLICT.*$").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("^INSERT INTO chain\\.address2blocks_test_relay_testchain \\(address, block_id\\) VALUES \\(\\$1, \\$2\\) ON CONFLICT \\(address, block_id\\) DO NOTHING$").WithArgs("5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty", "2").WillReturnResult(sqlmock.NewResult(0, 1))
	
	mock.ExpectCommit()

	// Second batch (item 2)
	mock.ExpectBegin()
	// For item 2: first blocks table, then address2blocks table
	mock.ExpectExec("^INSERT INTO chain\\.blocks_test_relay_testchain \\(block_id, created_at, hash, parent_hash, state_root, extrinsics_root, author_id, finalized, on_initialize, on_finalize, logs, extrinsics\\) VALUES \\(.*\\) ON CONFLICT.*$").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("^INSERT INTO chain\\.address2blocks_test_relay_testchain \\(address, block_id\\) VALUES \\(\\$1, \\$2\\) ON CONFLICT \\(address, block_id\\) DO NOTHING$").WithArgs("5DAAnrj7VHTznn2AWBemMuyBwZWs6FNFjdyVXUeYum3PTXFy", "3").WillReturnResult(sqlmock.NewResult(0, 1))
	
	mock.ExpectCommit()

	// Create database with mock
	database := NewSQLDatabase(db)

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

	// Save data to database
	err = database.Save(testData, testConfig)
	assert.NoError(t, err, "Should not error when saving data")

	// Wait for batch processing to complete
	<-done
	wg.Wait()

	// Verify all expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err, "All SQL expectations should be met")
}

func TestBatchFlushOnSize(t *testing.T) {
	// Create a mock database connection
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Create test data
	testData := []BlockData{
		{
			ID:             "1",
			Timestamp:      time.Now(),
			Hash:           "hash1",
			ParentHash:     "parenthash1",
			StateRoot:      "stateroot1",
			ExtrinsicsRoot: "extrinsicsroot1",
			AuthorID:       "author1",
			Finalized:      true,
			OnInitialize:   json.RawMessage(`{"test": true}`),
			OnFinalize:     json.RawMessage(`{"test": true}`),
			Logs:           json.RawMessage(`{"test": true}`),
			Extrinsics: json.RawMessage(`[
				{
					"method": "timestamp.set",
					"now": 1234567890,
					"signer_id": "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY"
				}
			]`),
		},
		{
			ID:             "2",
			Timestamp:      time.Now(),
			Hash:           "hash2",
			ParentHash:     "parenthash2",
			StateRoot:      "stateroot2",
			ExtrinsicsRoot: "extrinsicsroot2",
			AuthorID:       "author2",
			Finalized:      true,
			OnInitialize:   json.RawMessage(`{"test": true}`),
			OnFinalize:     json.RawMessage(`{"test": true}`),
			Logs:           json.RawMessage(`{"test": true}`),
			Extrinsics: json.RawMessage(`[
				{
					"method": "timestamp.set",
					"now": 1234567890,
					"account_id": "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty"
				}
			]`),
		},
	}

	// Create config with small batch size to ensure size triggers the flush
	testConfig := Config{
		Relaychain:   "test_relay",
		Chain:        "testchain",
		BatchSize:    2, // Set batch size to 2 to force a flush after items 0 and 1
		FlushTimeout: 10 * time.Second, // Long timeout to ensure size triggers flush, not time
	}

	// Set up expectations for the batch
	mock.ExpectBegin()
	
	// For item 0: first blocks table, then address2blocks table
	mock.ExpectExec("^INSERT INTO chain\\.blocks_test_relay_testchain \\(block_id, created_at, hash, parent_hash, state_root, extrinsics_root, author_id, finalized, on_initialize, on_finalize, logs, extrinsics\\) VALUES \\(.*\\) ON CONFLICT.*$").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("^INSERT INTO chain\\.address2blocks_test_relay_testchain \\(address, block_id\\) VALUES \\(\\$1, \\$2\\) ON CONFLICT \\(address, block_id\\) DO NOTHING$").WithArgs("5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY", "1").WillReturnResult(sqlmock.NewResult(0, 1))
	
	// For item 1: first blocks table, then address2blocks table
	mock.ExpectExec("^INSERT INTO chain\\.blocks_test_relay_testchain \\(block_id, created_at, hash, parent_hash, state_root, extrinsics_root, author_id, finalized, on_initialize, on_finalize, logs, extrinsics\\) VALUES \\(.*\\) ON CONFLICT.*$").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("^INSERT INTO chain\\.address2blocks_test_relay_testchain \\(address, block_id\\) VALUES \\(\\$1, \\$2\\) ON CONFLICT \\(address, block_id\\) DO NOTHING$").WithArgs("5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty", "2").WillReturnResult(sqlmock.NewResult(0, 1))
	
	mock.ExpectCommit()

	// Create database with mock
	database := NewSQLDatabase(db)

	// Create a wait group to wait for async processing
	var wg sync.WaitGroup
	wg.Add(1)

	// Use a channel to signal when we should check results
	done := make(chan struct{})
	go func() {
		defer wg.Done()
		time.Sleep(200 * time.Millisecond) // Give time for batch processing
		done <- struct{}{}
	}()

	// Save first two items to trigger a batch flush due to size
	err = database.Save(testData, testConfig)
	assert.NoError(t, err, "Should not error when saving data")

	// Wait for batch processing to complete
	<-done
	wg.Wait()

	// Verify all expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err, "All SQL expectations should be met")
}

func TestFlushOnClose(t *testing.T) {
	// Create a mock database connection
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Create test data with a single item
	testData := []BlockData{
		{
			ID:             "1",
			Timestamp:      time.Now(),
			Hash:           "hash1",
			ParentHash:     "parenthash1",
			StateRoot:      "stateroot1",
			ExtrinsicsRoot: "extrinsicsroot1",
			AuthorID:       "author1",
			Finalized:      true,
			OnInitialize:   json.RawMessage(`{"test": true}`),
			OnFinalize:     json.RawMessage(`{"test": true}`),
			Logs:           json.RawMessage(`{"test": true}`),
			Extrinsics: json.RawMessage(`[
				{
					"method": "timestamp.set",
					"now": 1234567890,
					"signer_id": "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY"
				}
			]`),
		},
	}

	// Create config with large batch size to ensure no automatic flushing
	testConfig := Config{
		Relaychain:   "test_relay",
		Chain:        "testchain",
		BatchSize:    100, // Large batch size to prevent automatic flushing
		FlushTimeout: 10 * time.Second,
	}

	// Set up expectations for the batch
	mock.ExpectBegin()
	
	// For item 0: first blocks table, then address2blocks table
	mock.ExpectExec("^INSERT INTO chain\\.blocks_test_relay_testchain \\(block_id, created_at, hash, parent_hash, state_root, extrinsics_root, author_id, finalized, on_initialize, on_finalize, logs, extrinsics\\) VALUES \\(.*\\) ON CONFLICT.*$").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("^INSERT INTO chain\\.address2blocks_test_relay_testchain \\(address, block_id\\) VALUES \\(\\$1, \\$2\\) ON CONFLICT \\(address, block_id\\) DO NOTHING$").WithArgs("5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY", "1").WillReturnResult(sqlmock.NewResult(0, 1))
	
	mock.ExpectCommit()
	mock.ExpectClose()

	// Create database with mock
	database := NewSQLDatabase(db)

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

	// Save data to database (shouldn't trigger flush yet due to large batch size)
	err = database.Save(testData, testConfig)
	assert.NoError(t, err, "Should not error when saving data")

	// Close database to trigger flush
	err = database.Close()
	assert.NoError(t, err, "Should not error when closing database")

	// Wait for batch processing to complete
	<-done
	wg.Wait()

	// Verify all expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err, "All SQL expectations should be met")
}
