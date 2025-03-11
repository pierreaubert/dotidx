package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// Database defines the interface for database operations
type Database interface {
	CreateTable(config Config) error
	Save(items []BlockData, config Config) error
	GetExistingBlocks(startRange, endRange int, config Config) (map[int]bool, error)
	Ping() error
	GetStats() *MetricsStats
	DoUpgrade(config Config) error
}

// version of schema for upgrade
const SQLDatabaseSchemaVersion = 2

// SQLDatabase implements Database using SQL
type SQLDatabase struct {
	db      *sql.DB
	metrics *Metrics
}

// NewSQLDatabase creates a new Database instance
func NewSQLDatabase(db *sql.DB) *SQLDatabase {
	return &SQLDatabase{
		db:      db,
		metrics: NewMetrics("Postgres"),
	}
}

func (s *SQLDatabase) DoUpgrade(config Config) error {

	// create dotlake version table to track migrations
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS dotlake_version (
			version_id INTEGER NOT NULL,
			timestamp TIMESTAMP WITHOUT TIME ZONE,
                        CONSTRAINT dotlake_version_pkey PRIMARY KEY (version_id)
		)
        `)
	if err != nil {
		return fmt.Errorf("error creating table: %w", err)
	}

	return nil
}

// CreateTable creates the necessary tables if they don't exist
func (s *SQLDatabase) CreateTable(config Config) error {
	const schemaName = "public"
	chainName := sanitizeChainName(config.Chain, config.Relaychain)

	// Create blocks table
	blocksTable := fmt.Sprintf("%s.blocks_%s_%s", schemaName, strings.ToLower(config.Relaychain), chainName)

	// Create the blocks table
	_, err := s.db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			block_id INTEGER NOT NULL,
			timestamp TIMESTAMP WITHOUT TIME ZONE,
			hash TEXT,
			parent_hash TEXT,
			state_root TEXT,
			extrinsics_root TEXT,
			author_id TEXT,
			finalized BOOLEAN,
			on_initialize JSONB,
			on_finalize JSONB,
			logs JSONB,
			extrinsics JSONB,
                        CONSTRAINT blocks_polkadot_polkadot_pkey PRIMARY KEY (block_id)
		)
	`, blocksTable))
	if err != nil {
		return fmt.Errorf("error creating blocks table: %w", err)
	}

	// Create index on address column
	_, err = s.db.Exec(fmt.Sprintf(`
                CREATE INDEX IF NOT EXISTS extrinsincs_idx
                ON %s USING gin(extrinsics jsonb_path_ops)
                WITH (fastupdate=True)
                TABLESPACE pg_default;
	`, blocksTable))
	if err != nil {
		return fmt.Errorf("error creating index on address column: %w", err)
	}

	// Create address to blocks mapping table
	address2blocksTable := fmt.Sprintf("address2blocks_%s_%s", strings.ToLower(config.Relaychain), chainName)
	_, err = s.db.Exec(fmt.Sprintf(`
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
	_, err = s.db.Exec(fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s_address_idx ON %s (address)
	`, address2blocksTable, address2blocksTable))
	if err != nil {
		return fmt.Errorf("error creating index on address column: %w", err)
	}

	return nil
}

// Save saves the given items to the database
func (s *SQLDatabase) Save(items []BlockData, config Config) error {
	if len(items) == 0 {
		return nil
	}

	start := time.Now()
	defer func(start time.Time) {
		go func(start time.Time, err error) {
			s.metrics.RecordLatency(start, len(items), 0, err)
		}(start, nil)
	}(start)

	// Sanitize chain name
	chainName := sanitizeChainName(config.Chain, config.Relaychain)

	// Get table names
	blocksTable := fmt.Sprintf("blocks_%s_%s", strings.ToLower(config.Relaychain), chainName)
	address2blocksTable := fmt.Sprintf("address2blocks_%s_%s", strings.ToLower(config.Relaychain), chainName)

	// Begin transaction
	tx, err := s.db.Begin()
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
			block_id, timestamp, hash, parent_hash, state_root, extrinsics_root,
			author_id, finalized, on_initialize, on_finalize, logs, extrinsics
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (block_id) DO UPDATE SET
			timestamp = EXCLUDED.timestamp,
			hash = EXCLUDED.hash,
			parent_hash = EXCLUDED.parent_hash,
			state_root = EXCLUDED.state_root,
			extrinsics_root = EXCLUDED.extrinsics_root,
			author_id = EXCLUDED.author_id,
			finalized = EXCLUDED.finalized,
			on_initialize = EXCLUDED.on_initialize,
			on_finalize = EXCLUDED.on_finalize,
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

// GetExistingBlocks retrieves a list of block IDs that already exist in the database
func (s *SQLDatabase) GetExistingBlocks(startRange, endRange int, config Config) (map[int]bool, error) {
	// Sanitize chain name
	chainName := sanitizeChainName(config.Chain, config.Relaychain)

	// Create blocks table name
	blocksTable := fmt.Sprintf("blocks_%s_%s", strings.ToLower(config.Relaychain), chainName)

	// Query for existing blocks
	rows, err := s.db.Query(fmt.Sprintf("SELECT block_id FROM %s WHERE block_id BETWEEN $1 AND $2", blocksTable), startRange, endRange)
	if err != nil {
		return nil, fmt.Errorf("error querying for existing blocks: %w", err)
	}
	defer rows.Close()

	// Create map of existing blocks
	existingBlocks := make(map[int]bool)
	for rows.Next() {
		var blockID int
		if err := rows.Scan(&blockID); err != nil {
			return nil, fmt.Errorf("error scanning block ID: %w", err)
		}
		existingBlocks[blockID] = true
	}

	return existingBlocks, nil
}

func (s *SQLDatabase) Ping() error {
	return s.db.Ping()
}

func (s *SQLDatabase) GetStats() *MetricsStats {
	return s.metrics.GetStats()
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
