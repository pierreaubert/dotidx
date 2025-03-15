package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestQueryTemplateCaching(t *testing.T) {
	// Create a mock database connection
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Create an SQLDatabase instance
	sqlDB := NewSQLDatabase(db)

	// Define the table name and query for testing
	tableName := "test_table"
	queryTemplate := "SELECT * FROM %s WHERE id = $1"

	// First access should create a new entry in the cache
	mock.ExpectExec(fmt.Sprintf("SELECT \\* FROM %s WHERE id = \\$1", tableName)).
		WithArgs(1).WillReturnResult(sqlmock.NewResult(1, 1))

	template, err := sqlDB.getQueryTemplate(tableName, fmt.Sprintf(queryTemplate, tableName))
	assert.NoError(t, err, "Should not error when getting a query template")
	assert.Equal(t, fmt.Sprintf(queryTemplate, tableName), template, "Template should match")

	// Check that the template was cached
	assert.Equal(t, 1, len(sqlDB.queryTemplates), "Should have one cached template")

	// Second access should use the cached template
	template2, err := sqlDB.getQueryTemplate(tableName, "This should not matter as it's cached")
	assert.NoError(t, err, "Should not error when getting a cached query template")
	assert.Equal(t, template, template2, "Second template should match the first")

	// Still only one entry in cache
	assert.Equal(t, 1, len(sqlDB.queryTemplates), "Should still have only one cached template")
}

func TestQueryTemplateCachingPerformance(t *testing.T) {
	// Create a mock database connection
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Create an SQLDatabase instance
	sqlDB := NewSQLDatabase(db)

	// Define the table names and query for testing
	baseTableName := "test_table"
	queryTemplate := "INSERT INTO %s (id, name, value) VALUES ($1, $2, $3) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, value = EXCLUDED.value"

	// Test with a large number of similar but distinct table names
	const numTables = 1000
	
	// First pass - should be building and caching templates
	startFirst := time.Now()
	for i := 0; i < numTables; i++ {
		tableName := fmt.Sprintf("%s_%d", baseTableName, i)
		formattedQuery := fmt.Sprintf(queryTemplate, tableName)
		_, err := sqlDB.getQueryTemplate(tableName, formattedQuery)
		assert.NoError(t, err, "Should not error when getting a query template")
	}
	elapsedFirst := time.Since(startFirst)

	// Second pass - should be using cached templates
	startSecond := time.Now()
	for i := 0; i < numTables; i++ {
		tableName := fmt.Sprintf("%s_%d", baseTableName, i)
		_, err := sqlDB.getQueryTemplate(tableName, "This doesn't matter as it's cached")
		assert.NoError(t, err, "Should not error when getting a cached query template")
	}
	elapsedSecond := time.Since(startSecond)

	// The second pass should be significantly faster
	t.Logf("First pass (create): %v, Second pass (cache hit): %v", elapsedFirst, elapsedSecond)
	assert.Less(t, elapsedSecond.Nanoseconds(), elapsedFirst.Nanoseconds(), 
		"Second pass with cache should be faster than first pass")

	// Verify we have the expected number of cached templates
	assert.Equal(t, numTables, len(sqlDB.queryTemplates), "Should have cached all templates")
}
