package main

import (
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

	// Create test configuration
	config := Config{
		BatchSize:    5,
		FlushTimeout: 500 * time.Millisecond,
		Relaychain:   "test_relay",
		Chain:        "test_chain",
	}

	// Create an SQLDatabase instance with the mock
	sqlDB := NewSQLDatabase(db)

	// Setup expectations for the prepared statements and executions
	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO")
	mock.ExpectPrepare("INSERT INTO")

	// Expect executions for batch items
	for i := 0; i < 3; i++ {
		mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(1, 1))
	}

	mock.ExpectCommit()

	// Create test block data
	testBlocks := []BlockData{
		{
			ID:             "1",
			Hash:           "0x1234567890abcdef1234567890abcdef",
			ParentHash:     "0xabcdef1234567890abcdef1234567890",
			StateRoot:      "0x1234567890abcdef1234567890abcdef",
			ExtrinsicsRoot: "0xabcdef1234567890abcdef1234567890",
			AuthorID:       "0x1234567890",
			Finalized:      true,
			Extrinsics:     []byte(`{"timestamp": "2023-01-01T00:00:00Z"}`),
		},
		{
			ID:             "2",
			Hash:           "0x2345678901abcdef2345678901abcdef",
			ParentHash:     "0x1234567890abcdef1234567890abcdef",
			StateRoot:      "0x2345678901abcdef2345678901abcdef",
			ExtrinsicsRoot: "0x2345678901abcdef2345678901abcdef",
			AuthorID:       "0x2345678901",
			Finalized:      true,
			Extrinsics:     []byte(`{"timestamp": "2023-01-01T00:00:01Z"}`),
		},
		{
			ID:             "3",
			Hash:           "0x3456789012abcdef3456789012abcdef",
			ParentHash:     "0x2345678901abcdef2345678901abcdef",
			StateRoot:      "0x3456789012abcdef3456789012abcdef",
			ExtrinsicsRoot: "0x3456789012abcdef3456789012abcdef",
			AuthorID:       "0x3456789012",
			Finalized:      true,
			Extrinsics:     []byte(`{"timestamp": "2023-01-01T00:00:02Z"}`),
		},
	}

	// Setup a wait group to wait for async operations
	var wg sync.WaitGroup
	wg.Add(1)
	
	// Use a timer to stop waiting after timeout
	done := make(chan struct{})
	go func() {
		defer wg.Done()
		time.Sleep(700 * time.Millisecond)
		done <- struct{}{}
	}()

	// Add items to the batch (not enough to trigger auto-flush)
	err = sqlDB.Save(testBlocks, config)
	assert.NoError(t, err, "Should not error when adding items to batch")

	// Wait for the flush timeout
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

	// Create test configuration with small batch size
	config := Config{
		BatchSize:    2, // Small batch size to trigger flush
		FlushTimeout: 10 * time.Second, // Long timeout to ensure size triggers flush, not time
		Relaychain:   "test_relay",
		Chain:        "test_chain",
	}

	// Create an SQLDatabase instance with the mock
	sqlDB := NewSQLDatabase(db)

	// Setup expectations for the prepared statements and executions
	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO")
	mock.ExpectPrepare("INSERT INTO")

	// Expect executions for batch items
	for i := 0; i < 2; i++ {
		mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(1, 1))
	}

	mock.ExpectCommit()

	// Create test block data that will exceed batch size
	testBlocks := []BlockData{
		{
			ID:             "1",
			Hash:           "0x1234567890abcdef1234567890abcdef",
			ParentHash:     "0xabcdef1234567890abcdef1234567890",
			StateRoot:      "0x1234567890abcdef1234567890abcdef",
			ExtrinsicsRoot: "0xabcdef1234567890abcdef1234567890",
			AuthorID:       "0x1234567890",
			Finalized:      true,
			Extrinsics:     []byte(`{"timestamp": "2023-01-01T00:00:00Z"}`),
		},
		{
			ID:             "2",
			Hash:           "0x2345678901abcdef2345678901abcdef",
			ParentHash:     "0x1234567890abcdef1234567890abcdef",
			StateRoot:      "0x2345678901abcdef2345678901abcdef",
			ExtrinsicsRoot: "0x2345678901abcdef2345678901abcdef",
			AuthorID:       "0x2345678901",
			Finalized:      true,
			Extrinsics:     []byte(`{"timestamp": "2023-01-01T00:00:01Z"}`),
		},
	}

	// Setup a wait group to wait for async operations
	var wg sync.WaitGroup
	wg.Add(1)
	
	// Use a timer to stop waiting after timeout
	done := make(chan struct{})
	go func() {
		defer wg.Done()
		time.Sleep(300 * time.Millisecond)
		done <- struct{}{}
	}()

	// Add items to the batch (should trigger auto-flush based on size)
	err = sqlDB.Save(testBlocks, config)
	assert.NoError(t, err, "Should not error when adding items to batch")

	// Wait for a short time to allow async processing
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

	// Create test configuration
	config := Config{
		BatchSize:    10, // Large batch size to prevent auto-flush
		FlushTimeout: 10 * time.Second, // Long timeout to prevent timeout flush
		Relaychain:   "test_relay",
		Chain:        "test_chain",
	}

	// Create an SQLDatabase instance with the mock
	sqlDB := NewSQLDatabase(db)

	// Setup expectations for the prepared statements and executions
	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO")
	mock.ExpectPrepare("INSERT INTO")

	// Expect executions for batch item
	mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()
	
	// Expect database close
	mock.ExpectClose()

	// Create test block data
	testBlocks := []BlockData{
		{
			ID:             "1",
			Hash:           "0x1234567890abcdef1234567890abcdef",
			ParentHash:     "0xabcdef1234567890abcdef1234567890",
			StateRoot:      "0x1234567890abcdef1234567890abcdef",
			ExtrinsicsRoot: "0xabcdef1234567890abcdef1234567890",
			AuthorID:       "0x1234567890",
			Finalized:      true,
			Extrinsics:     []byte(`{"timestamp": "2023-01-01T00:00:00Z"}`),
		},
	}

	// Add item to the batch (not enough to trigger auto-flush)
	err = sqlDB.Save(testBlocks, config)
	assert.NoError(t, err, "Should not error when adding items to batch")

	// Close the database (should flush remaining items)
	err = sqlDB.Close()
	assert.NoError(t, err, "Should not error when closing database")

	// Verify all expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err, "All SQL expectations should be met")
}
