package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pierreaubert/dotidx/dix"
)

// GetDatabaseInfoActivity retrieves information about all indexed chains
func (a *Activities) GetDatabaseInfoActivity(ctx context.Context) ([]ChainInfo, error) {
	start := time.Now()
	log.Printf("[Activity] Getting database info")

	if a.database == nil {
		return nil, fmt.Errorf("database not configured in activities")
	}

	dbInfos, err := a.database.GetDatabaseInfo()
	if err != nil {
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("GetDatabaseInfo", "error")
		}
		return nil, fmt.Errorf("failed to get database info: %w", err)
	}

	// Convert to ChainInfo format
	chains := make([]ChainInfo, len(dbInfos))
	for i, info := range dbInfos {
		chains[i] = ChainInfo{
			RelayChain: info.Relaychain,
			Chain:      info.Chain,
		}
	}

	if a.metrics != nil {
		a.metrics.RecordActivityExecution("GetDatabaseInfo", "success")
		a.metrics.RecordActivityDuration("GetDatabaseInfo", time.Since(start))
	}

	log.Printf("[Activity] Found %d indexed chains", len(chains))
	return chains, nil
}

// CheckQueryResultExistsActivity checks if a query result already exists
func (a *Activities) CheckQueryResultExistsActivity(ctx context.Context,
	relayChain, chain, queryName string, year, month int) (bool, error) {

	start := time.Now()
	log.Printf("[Activity] Checking if query result exists: %s/%s, query=%s, year=%d, month=%d",
		relayChain, chain, queryName, year, month)

	if a.database == nil {
		return false, fmt.Errorf("database not configured in activities")
	}

	// Try to read the query result timestamp
	// If it exists, the timestamp will be non-zero
	timestamp, err := a.database.ReadTimeNamedQuery(ctx, relayChain, chain, queryName, year, month)
	if err != nil {
		// Error means the result doesn't exist or there was a database error
		// We'll treat this as "doesn't exist" to allow retry
		log.Printf("[Activity] Query result not found or error: %v", err)
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("CheckQueryResultExists", "success")
			a.metrics.RecordActivityDuration("CheckQueryResultExists", time.Since(start))
		}
		return false, nil
	}

	// Check if timestamp is non-zero (result exists)
	exists := !timestamp.IsZero()

	if a.metrics != nil {
		a.metrics.RecordActivityExecution("CheckQueryResultExists", "success")
		a.metrics.RecordActivityDuration("CheckQueryResultExists", time.Since(start))
	}

	log.Printf("[Activity] Query result exists: %v (timestamp: %v)", exists, timestamp)
	return exists, nil
}

// ExecuteAndStoreNamedQueryActivity executes a registered query and stores the result
func (a *Activities) ExecuteAndStoreNamedQueryActivity(ctx context.Context,
	relayChain, chain, queryName string, year, month int) error {

	start := time.Now()
	log.Printf("[Activity] Executing and storing query: %s/%s, query=%s, year=%d, month=%d",
		relayChain, chain, queryName, year, month)

	if a.database == nil {
		return fmt.Errorf("database not configured in activities")
	}

	// Execute and store the query
	err := a.database.ExecuteAndStoreNamedQuery(ctx, relayChain, chain, queryName, year, month)
	if err != nil {
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("ExecuteAndStoreNamedQuery", "error")
		}
		return fmt.Errorf("failed to execute and store query: %w", err)
	}

	if a.metrics != nil {
		a.metrics.RecordActivityExecution("ExecuteAndStoreNamedQuery", "success")
		a.metrics.RecordActivityDuration("ExecuteAndStoreNamedQuery", time.Since(start))
	}

	log.Printf("[Activity] Successfully executed and stored query (duration: %v)", time.Since(start))
	return nil
}

// RegisterDefaultQueriesActivity registers the default queries used by dixcron
func (a *Activities) RegisterDefaultQueriesActivity(ctx context.Context) error {
	start := time.Now()
	log.Printf("[Activity] Registering default queries")

	// Query 1: Total blocks in month
	totalBlocksQuery := `
		SELECT COUNT(*) as total_blocks
		FROM chain.{{.Relaychain}}_{{.Chain}}_blocks
		WHERE created_at >= '{{.Year}}-{{.Month}}-01'
		  AND created_at < '{{.Year}}-{{.Month}}-01'::date + INTERVAL '1 month'
	`

	err := dix.RegisterQuery("total_blocks_in_month", totalBlocksQuery,
		"Count total blocks indexed in a given month")
	if err != nil && err.Error() != "query with name 'total_blocks_in_month' already registered" {
		log.Printf("[Activity] Warning: failed to register total_blocks_in_month: %v", err)
	}

	// Query 2: Total addresses in month
	totalAddressesQuery := `
		WITH month_bounds AS (
			SELECT
				MIN(block_id) as min_block,
				MAX(block_id) as max_block
			FROM chain.{{.Relaychain}}_{{.Chain}}_blocks
			WHERE created_at >= '{{.Year}}-{{.Month}}-01'
			  AND created_at < '{{.Year}}-{{.Month}}-01'::date + INTERVAL '1 month'
		)
		SELECT COUNT(DISTINCT address) as total_addresses
		FROM chain.{{.Relaychain}}_{{.Chain}}_address_to_blocks atb
		CROSS JOIN month_bounds mb
		WHERE atb.block_id >= mb.min_block
		  AND atb.block_id <= mb.max_block
	`

	err = dix.RegisterQuery("total_addresses_in_month", totalAddressesQuery,
		"Count unique addresses active in a given month")
	if err != nil && err.Error() != "query with name 'total_addresses_in_month' already registered" {
		log.Printf("[Activity] Warning: failed to register total_addresses_in_month: %v", err)
	}

	if a.metrics != nil {
		a.metrics.RecordActivityExecution("RegisterDefaultQueries", "success")
		a.metrics.RecordActivityDuration("RegisterDefaultQueries", time.Since(start))
	}

	log.Printf("[Activity] Default queries registered")
	return nil
}
