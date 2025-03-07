package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// createTable creates the necessary tables if they don't exist
func createTable(db *sql.DB, config Config) error {
	// Sanitize chain name
	chainName := sanitizeChainName(config.Chain, config.Relaychain)

	// Create blocks table
	blocksTable := fmt.Sprintf("blocks_%s_%s", strings.ToLower(config.Relaychain), chainName)

	// Create the blocks table
	_, err := db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			block_id INTEGER PRIMARY KEY,
			timestamp TIMESTAMP,
			hash TEXT,
			parenthash TEXT,
			state_root TEXT,
			extrinsics_root TEXT,
			author_id TEXT,
			finalized BOOLEAN,
			on_initialize JSONB,
			on_finalize JSONB,
			logs JSONB,
			extrinsics JSONB
		)
	`, blocksTable))
	if err != nil {
		return fmt.Errorf("error creating blocks table: %w", err)
	}

	// Create address to blocks mapping table
	address2blocksTable := fmt.Sprintf("address2blocks_%s_%s", strings.ToLower(config.Relaychain), chainName)
	_, err = db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			address TEXT,
			block_id INTEGER,
			PRIMARY KEY (address, block_id)
		)
	`, address2blocksTable))
	if err != nil {
		return fmt.Errorf("error creating address2blocks table: %w", err)
	}

	// Create index on address column
	_, err = db.Exec(fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s_address_idx ON %s (address)
	`, address2blocksTable, address2blocksTable))
	if err != nil {
		return fmt.Errorf("error creating index on address column: %w", err)
	}

	return nil
}

// saveToDatabase saves the given items to the database
func saveToDatabase(db *sql.DB, items []BlockData, config Config) error {
	if len(items) == 0 {
		return nil
	}

	// Sanitize chain name
	chainName := sanitizeChainName(config.Chain, config.Relaychain)

	// Get table names
	blocksTable := fmt.Sprintf("blocks_%s_%s", strings.ToLower(config.Relaychain), chainName)
	address2blocksTable := fmt.Sprintf("address2blocks_%s_%s", strings.ToLower(config.Relaychain), chainName)

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error beginning transaction: %w", err)
	}
	defer func() {
		if err != nil {
			rbErr := tx.Rollback()
			if rbErr != nil {
				log.Printf("Error rolling back transaction: %v", rbErr)
			}
		}
	}()

	// Prepare statement for blocks table
	blocksStmt, err := tx.Prepare(fmt.Sprintf(`
		INSERT INTO %s (
			block_id, timestamp, hash, parenthash, stateroot, extrinsicsroot,
			authorid, finalized, oninitialize, onfinalize, logs, extrinsics
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (block_id) DO UPDATE SET
			timestamp = EXCLUDED.timestamp,
			hash = EXCLUDED.hash,
			parenthash = EXCLUDED.parenthash,
			stateroot = EXCLUDED.stateroot,
			extrinsicsroot = EXCLUDED.extrinsicsroot,
			authorid = EXCLUDED.authorid,
			finalized = EXCLUDED.finalized,
			oninitialize = EXCLUDED.oninitialize,
			onfinalize = EXCLUDED.onfinalize,
			logs = EXCLUDED.logs,
			extrinsics = EXCLUDED.extrinsics
	`, blocksTable))
	if err != nil {
		return fmt.Errorf("error preparing statement for blocks table: %w", err)
	}
	defer blocksStmt.Close()

	// Prepare statement for address2blocks table
	address2blocksStmt, err := tx.Prepare(fmt.Sprintf(`
		INSERT INTO %s (address, block_id)
		VALUES ($1, $2)
		ON CONFLICT (address, block_id) DO NOTHING
	`, address2blocksTable))
	if err != nil {
		return fmt.Errorf("error preparing statement for address2blocks table: %w", err)
	}
	defer address2blocksStmt.Close()

	// Insert items
	for _, item := range items {
		// Insert into blocks table
		_, err = blocksStmt.Exec(
			item.ID,
			item.Timestamp,
			item.Hash,
			item.ParentHash,
			item.StateRoot,
			item.ExtrinsicsRoot,
			item.AuthorID,
			item.Finalized,
			item.OnInitialize,
			item.OnFinalize,
			item.Logs,
			item.Extrinsics,
		)
		if err != nil {
			return fmt.Errorf("error inserting into blocks table: %w", err)
		}

		// Extract addresses from extrinsics
		addresses, err := extractAddressesFromExtrinsics(item.Extrinsics)
		if err != nil {
			log.Printf("Warning: error extracting addresses from extrinsics: %v", err)
			continue
		}

		// Insert into address2blocks table
		for _, address := range addresses {
			_, err = address2blocksStmt.Exec(address, item.ID)
			if err != nil {
				return fmt.Errorf("error inserting into address2blocks table: %w", err)
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}

// sanitizeChainName removes non-alphanumeric characters and the relaychain name from the chain name
func sanitizeChainName(initialChainName, initialRelaychainName string) string {
	chainName := strings.ToLower(initialChainName)
	relaychainName := strings.ToLower(initialRelaychainName)

	// Remove non-alphanumeric characters (like hyphens)
	var result strings.Builder
	for _, char := range chainName {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			result.WriteRune(char)
		}
	}
	chainName = result.String()

	// Remove relaychain name if it's included in the chain name
	if initialChainName != initialRelaychainName {
		chainName = strings.ReplaceAll(chainName, relaychainName, "")
	}
	return chainName
}

// getExistingBlocks retrieves a list of block IDs that already exist in the database
func getExistingBlocks(db *sql.DB, startRange, endRange int, config Config) (map[int]bool, error) {
	// Sanitize chain name
	chainName := sanitizeChainName(config.Chain, config.Relaychain)

	// Create blocks table name
	blocksTable := fmt.Sprintf("blocks_%s_%s", strings.ToLower(config.Relaychain), chainName)

	// Query for existing blocks
	query := fmt.Sprintf("SELECT block_id FROM %s WHERE block_id BETWEEN $1 AND $2", blocksTable)
	rows, err := db.Query(query, startRange, endRange)
	if err != nil {
		return nil, fmt.Errorf("error querying existing blocks: %w", err)
	}
	defer rows.Close()

	// Create a map to store existing block IDs
	existingBlocks := make(map[int]bool)
	// Iterate through the results
	for rows.Next() {
		var blockID int
		if err := rows.Scan(&blockID); err != nil {
			return nil, fmt.Errorf("error scanning block ID: %w", err)
		}
		existingBlocks[blockID] = true
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return existingBlocks, nil
}

// monitorNewBlocks continuously monitors for new blocks and adds them to the database
func monitorNewBlocks(ctx context.Context, config Config, db *sql.DB, lastProcessedBlock int) {
	// Create a ticker that ticks every second
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Block monitor stopped due to context cancellation")
			return
		case <-ticker.C:
			// Fetch the current head block
			headBlock, err := fetchHeadBlock(config.SidecarURL)
			if err != nil {
				log.Printf("Error fetching head block: %v", err)
				continue
			}

			// Check if there are new blocks
			if headBlock > lastProcessedBlock {
				log.Printf("New blocks detected: %d to %d", lastProcessedBlock+1, headBlock)

				// Create array of block IDs to fetch
				blockIDs := make([]int, 0, headBlock-lastProcessedBlock)
				for id := lastProcessedBlock + 1; id <= headBlock; id++ {
					blockIDs = append(blockIDs, id)
				}

				// Fetch and process the new blocks
				blocks, err := fetchBlockRange(ctx, blockIDs, config.SidecarURL)
				if err != nil {
					log.Printf("Error fetching block range: %v", err)
					continue
				}

				// Save the blocks to the database
				if err := saveToDatabase(db, blocks, config); err != nil {
					log.Printf("Error saving blocks to database: %v", err)
					continue
				}

				// Update the last processed block
				lastProcessedBlock = headBlock
			}
		}
	}
}
