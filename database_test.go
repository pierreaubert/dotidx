package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/lib/pq"
)

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
