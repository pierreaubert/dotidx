package main

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Database defines the interface for database operations
type Database interface {
	CreateTable(config Config) error
	CreateIndex(config Config) error
	Save(items []BlockData, config Config) error
	GetExistingBlocks(startRange, endRange int, config Config) (map[int]bool, error)
	Ping() error
	GetStats() *MetricsStats
	DoUpgrade(config Config) error
	Close() error
}

// DBPoolConfig contains the configuration for the database connection pool
type DBPoolConfig struct {
	MaxOpenConns    int           // Maximum number of open connections
	MaxIdleConns    int           // Maximum number of idle connections
	ConnMaxLifetime time.Duration // Maximum lifetime of a connection
	ConnMaxIdleTime time.Duration // Maximum idle time of a connection
}

// DefaultDBPoolConfig returns a default configuration for the database connection pool
func DefaultDBPoolConfig() DBPoolConfig {
	return DBPoolConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}
}

// version of schema for upgrade
const SQLDatabaseSchemaVersion = 2

// SQLDatabase implements Database using SQL
type SQLDatabase struct {
	db      *sql.DB
	metrics *Metrics
	poolCfg DBPoolConfig
	
	// Query templates cache to avoid rebuilding queries each time
	queryTemplates     map[string]string
	queryTemplatesMutex sync.RWMutex
	
	// Batch processing
	batchMutex    sync.Mutex
	batchItems    []BlockData
	batchConfig   *Config
	batchTimer    *time.Timer
	batchShutdown chan struct{}
}

const schemaName = "chain"
const fastTablespaceRoot = "/dotlake/fast"
const fastTablespaceNumber = 4
const slowTablespaceRoot = "/dotlake/slow"
const slowTablespaceNumber = 6

// NewSQLDatabase creates a new Database instance
func NewSQLDatabase(db *sql.DB) *SQLDatabase {
	return NewSQLDatabaseWithPool(db, DefaultDBPoolConfig())
}

// NewSQLDatabaseWithPool creates a new Database instance with custom connection pool settings
func NewSQLDatabaseWithPool(db *sql.DB, poolCfg DBPoolConfig) *SQLDatabase {
	// Configure connection pool
	db.SetMaxOpenConns(poolCfg.MaxOpenConns)
	db.SetMaxIdleConns(poolCfg.MaxIdleConns)
	db.SetConnMaxLifetime(poolCfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(poolCfg.ConnMaxIdleTime)
	
	return &SQLDatabase{
		db:      db,
		metrics: NewMetrics("Postgres"),
		poolCfg: poolCfg,
		queryTemplates: make(map[string]string),
		batchShutdown: make(chan struct{}),
	}
}

// Close closes the database connection pool
func (s *SQLDatabase) Close() error {
	// Flush any pending batch and shut down the batch processor
	s.batchMutex.Lock()
	
	// Flush any remaining items
	if len(s.batchItems) > 0 && s.batchConfig != nil {
		items := s.batchItems
		config := *s.batchConfig
		
		s.batchItems = nil
		s.batchMutex.Unlock()
		
		// Process remaining items synchronously before closing
		err := s.saveBatch(items, config)
		if err != nil {
			log.Printf("Error flushing batch during shutdown: %v", err)
		}
	} else {
		s.batchMutex.Unlock()
	}
	
	// Signal the batch processor to stop
	if s.batchShutdown != nil {
		close(s.batchShutdown)
	}
	
	// Close the database
	return s.db.Close()
}

// getQueryTemplate returns a cached query template or creates a new one if it doesn't exist
func (s *SQLDatabase) getQueryTemplate(tableName, query string) (string, error) {
	// Try to get from cache first
	s.queryTemplatesMutex.RLock()
	template, exists := s.queryTemplates[tableName]
	s.queryTemplatesMutex.RUnlock()
	
	if exists {
		return template, nil
	}
	
	// If not in cache, prepare a new template
	s.queryTemplatesMutex.Lock()
	defer s.queryTemplatesMutex.Unlock()
	
	// Check again after acquiring write lock (double-checked locking)
	template, exists = s.queryTemplates[tableName]
	if exists {
		return template, nil
	}
	
	// Prepare the template
	template = query
	s.queryTemplates[tableName] = template
	return template, nil
}

func (s *SQLDatabase) DoUpgrade(config Config) error {

	// create dotlake version table to track migrations
	_, err := s.db.Exec(`
    CREATE TABLE IF NOT EXISTS dotlake_version (
	version_id INTEGER NOT NULL,
	timestamp TIMESTAMP(4) WITHOUT TIME ZONE,
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
	chainName := sanitizeChainName(config.Chain, config.Relaychain)

	blocksTable := fmt.Sprintf("%s.blocks_%s_%s", schemaName, strings.ToLower(config.Relaychain), chainName)

	// Create the blocks table
	template, err := s.getQueryTemplate(blocksTable, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s
(
  block_id        integer NOT NULL,
  created_at      timestamp(4) without time zone NOT NULL,
  hash            text COLLATE pg_catalog."default" NOT NULL,
  parent_hash     text COLLATE pg_catalog."default" NOT NULL,
  state_root      text COLLATE pg_catalog."default" NOT NULL,
  extrinsics_root text COLLATE pg_catalog."default" NOT NULL,
  author_id       text COLLATE pg_catalog."default" NOT NULL,
  finalized       boolean NOT NULL,
  on_initialize   jsonb,
  on_finalize     jsonb,
  logs            jsonb,
  extrinsics      jsonb,
  CONSTRAINT      block_pk PRIMARY KEY (block_id, created_at)
) PARTITION BY RANGE (created_at);
ALTER TABLE IF EXISTS %[1]s OWNER to dotlake;
REVOKE ALL ON TABLE %[1]s FROM PUBLIC;
GRANT SELECT ON TABLE %[1]s TO PUBLIC;
GRANT ALL ON TABLE %[1]s TO dotlake;
	`, blocksTable))
	if err != nil {
		return err
	}
	_, err = s.db.Exec(template)
	if err != nil {
		return fmt.Errorf("error creating blocks table: %s %w", template, err)
	}

	// CREATE PARTITIONS
	// Spread by month across the partition
	slow := 0
	fast := 0
	slow_or_fast := ""
	for year_idx := range 6 {
		year := 2020+year_idx
		if year >= time.Now().Year() {
			slow_or_fast = fmt.Sprintf("fast%d", fast);
			fast = min(fast+1,fastTablespaceNumber-1)
		} else {
			slow_or_fast = fmt.Sprintf("slow%d", slow);
			slow = min(slow+1,slowTablespaceNumber-1)
		}
		for month := range 12 {
			from_date := fmt.Sprintf("%04d-%02d-01 00:00:00.0000", year, month+1)
			to_date := fmt.Sprintf("%04d-%02d-01 00:00:00.0000", year, month+2)
			if month == 11 {
				to_date = fmt.Sprintf("%04d-%02d-01 00:00:00.0000", year+1, 1)
			}
			parts := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s_%04[2]d_%02[3]d PARTITION OF %[1]s
  FOR VALUES FROM (timestamp '%[5]s') TO (timestamp '%[6]s')
  TABLESPACE dotidx_%[7]s;
ALTER TABLE IF EXISTS %[1]s_%04[2]d_%02[3]d OWNER to dotlake;
REVOKE ALL ON TABLE %[1]s_%04[2]d_%02[3]d FROM PUBLIC;
GRANT SELECT ON TABLE %[1]s_%04[2]d_%02[3]d TO PUBLIC;
GRANT ALL ON TABLE %[1]s_%04[2]d_%02[3]d TO dotlake;
	`,
				blocksTable, // 1
				year,        // 2
				month+1,     // 3
				month+2,     // 4
				from_date,   // 5
				to_date,     // 6
				slow_or_fast,// 7
			)
			template, err = s.getQueryTemplate(blocksTable, parts)
			if err != nil {
				return err
			}
			_, err = s.db.Exec(template)
			if err != nil {
				return fmt.Errorf("error : %w", err)
			}
		}
	}

	address2blocksTable := fmt.Sprintf(
		"%s.address2blocks_%s_%s",
		schemaName,
		strings.ToLower(config.Relaychain),
		chainName,
	)
	template, err = s.getQueryTemplate(address2blocksTable, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
     address TEXT,
     block_id INTEGER,
     PRIMARY KEY (address, block_id)
) PARTITION BY HASH(address);
ALTER TABLE IF EXISTS %[1]s OWNER to dotlake;
REVOKE ALL ON TABLE %[1]s FROM PUBLIC;
GRANT SELECT ON TABLE %[1]s TO PUBLIC;
GRANT ALL ON TABLE %[1]s TO dotlake;
	`, address2blocksTable))
	if err != nil {
		return err
	}
	_, err = s.db.Exec(template)
	if err != nil {
		return fmt.Errorf("error creating address2blocks table: %w", err)
	}

	// CREATE PARTITIONS
	// spread across fast disks to improve access time
	for fast := range fastTablespaceNumber {
		parts := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s_%1[2]d PARTITION OF %[1]s
  FOR VALUES WITH (modulus %[3]d, remainder %[2]d)
  TABLESPACE dotidx_fast%[2]d;
ALTER TABLE IF EXISTS %[1]s_%1[2]d OWNER to dotlake;
REVOKE ALL ON TABLE %[1]s_%1[2]d FROM PUBLIC;
GRANT SELECT ON TABLE %[1]s_%1[2]d TO PUBLIC;
GRANT ALL ON TABLE %[1]s_%1[2]d TO dotlake;
	`,
			address2blocksTable,  // 1
			fast        ,         // 2
			fastTablespaceNumber, // 3
			)
		template, err = s.getQueryTemplate(address2blocksTable, parts)
		if err != nil {
			return err
		}
		_, err = s.db.Exec(template)
		if err != nil {
			return fmt.Errorf("error : %w", err)
		}
	}

	return nil
}

// TODO: adapt to the new partionning
// when tables are full (a month) they are immutable so we can write the index once and forall
func (s *SQLDatabase) CreateIndex(config Config) error {
	chainName := sanitizeChainName(config.Chain, config.Relaychain)

	blocksTable := fmt.Sprintf("%s.blocks_%s_%s", schemaName, strings.ToLower(config.Relaychain), chainName)
	template, err := s.getQueryTemplate(blocksTable, fmt.Sprintf(`
                CREATE INDEX IF NOT EXISTS extrinsincs_idx
                ON %s USING gin(extrinsics jsonb_path_ops)
                WITH (fastupdate=True)
                TABLESPACE pg_default;
	`, blocksTable))
	if err != nil {
		return err
	}
	_, err = s.db.Exec(template)
	if err != nil {
		return fmt.Errorf("error creating index on address column: %w", err)
	}

	address2blocksTable := fmt.Sprintf("address2blocks_%s_%s", strings.ToLower(config.Relaychain), chainName)
	template, err = s.getQueryTemplate(address2blocksTable, fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s_address_idx ON %s (address)
	`, address2blocksTable, address2blocksTable))
	if err != nil {
		return err
	}
	_, err = s.db.Exec(template)
	if err != nil {
		return fmt.Errorf("error creating index on address column: %w", err)
	}

	return nil
}

func extractTimestamp(extrinsics []byte) (ts string, err error) {
	const defaultTimestamp = "0001-01-01 00:00:00.0000"
	re := regexp.MustCompile("\"now\"[ ]*[:][ ]*\"[0-9]+\"")
	texts := re.FindAllString(string(extrinsics), 1)
	if len(texts) == 0 {
		return defaultTimestamp, fmt.Errorf("cannot find \"now\" in extrinsics: %w", err)
	}
	stexts := strings.Split(texts[0], "\"")
	if len(stexts) != 5 {
		return defaultTimestamp, fmt.Errorf("cannot find timestamp in extrinsics: len is %d", len(stexts))
	}
	millis, err := strconv.ParseInt(stexts[3], 10, 64)
	if err != nil {
		return defaultTimestamp, fmt.Errorf("cannot convert timestamp to milliseconds: %w", err)
	}
	ts = time.UnixMilli(millis).Format("2006-01-02 15:04:05.0000")
	return
}

// Save adds the given items to the batch for asynchronous processing
func (s *SQLDatabase) Save(items []BlockData, config Config) error {
	if len(items) == 0 {
		return nil
	}

	s.batchMutex.Lock()
	
	// Initialize batch if this is the first call
	if s.batchConfig == nil {
		configCopy := config
		s.batchConfig = &configCopy
		
		// Start a background goroutine to flush the batch after the timeout
		go s.startBatchProcessor()
	}
	
	// Add items to the batch
	s.batchItems = append(s.batchItems, items...)
	
	// Check if we've reached the batch size
	flushRequired := len(s.batchItems) >= config.BatchSize
	
	// Handle timer reset logic
	if !flushRequired && s.batchTimer == nil {
		s.batchTimer = time.AfterFunc(config.FlushTimeout, func() {
			s.flushBatch()
		})
	}
	
	// Unlock before potentially flushing to avoid deadlocks
	s.batchMutex.Unlock()
	
	// Flush if needed after unlocking
	if flushRequired {
		return s.flushBatch()
	}
	
	return nil
}

// startBatchProcessor runs a goroutine that periodically flushes the batch
func (s *SQLDatabase) startBatchProcessor() {
	for {
		select {
		case <-s.batchShutdown:
			return
		case <-time.After(5 * time.Second): // Check periodically
			// Try to acquire lock - don't block indefinitely
			if !s.tryLock(100 * time.Millisecond) {
				continue
			}
			
			// Only flush if we have items and no active timer
			if len(s.batchItems) > 0 && s.batchTimer == nil {
				// Unlock before flushing to avoid deadlocks
				s.batchMutex.Unlock()
				
				err := s.flushBatch()
				if err != nil {
					log.Printf("Error in periodic batch flush: %v", err)
				}
			} else {
				s.batchMutex.Unlock()
			}
		}
	}
}

// tryLock attempts to acquire the mutex lock with a timeout
func (s *SQLDatabase) tryLock(timeout time.Duration) bool {
	c := make(chan struct{}, 1)
	go func() {
		s.batchMutex.Lock()
		c <- struct{}{}
	}()
	
	select {
	case <-c:
		return true
	case <-time.After(timeout):
		return false
	}
}

// flushBatch processes all batched items and saves them to the database
func (s *SQLDatabase) flushBatch() error {
	// Try to acquire lock with a timeout
	if !s.tryLock(100 * time.Millisecond) {
		return fmt.Errorf("could not acquire lock for flushing batch")
	}
	
	// If there are no items or no config, return early
	if len(s.batchItems) == 0 || s.batchConfig == nil {
		s.batchMutex.Unlock()
		return nil
	}
	
	items := s.batchItems
	config := *s.batchConfig
	
	// Clear the batch
	s.batchItems = nil
	
	// Stop the timer if it's running
	if s.batchTimer != nil {
		s.batchTimer.Stop()
		s.batchTimer = nil
	}
	
	// Unlock before processing to avoid deadlocks
	s.batchMutex.Unlock()
	
	// Process the items in a separate goroutine
	go func(items []BlockData, config Config) {
		err := s.saveBatch(items, config)
		if err != nil {
			log.Printf("Error saving batch: %v", err)
		}
	}(items, config)
	
	return nil
}

// saveBatch saves the given items to the database immediately
func (s *SQLDatabase) saveBatch(items []BlockData, config Config) error {
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
	blocksTable := fmt.Sprintf("%s.blocks_%s_%s", schemaName, strings.ToLower(config.Relaychain), chainName)
	address2blocksTable := fmt.Sprintf("%s.address2blocks_%s_%s", schemaName, strings.ToLower(config.Relaychain), chainName)

	// Create insert query templates
	template, err := s.getQueryTemplate(blocksTable, fmt.Sprintf(`
		INSERT INTO %s (
			block_id, created_at, hash, parent_hash, state_root, extrinsics_root,
			author_id, finalized, on_initialize, on_finalize, logs, extrinsics
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (block_id, created_at) DO UPDATE SET
			created_at = EXCLUDED.created_at,
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
		return err
	}
	
	template2, err := s.getQueryTemplate(address2blocksTable, fmt.Sprintf(`
		INSERT INTO %s (address, block_id)
		VALUES ($1, $2)
		ON CONFLICT (address, block_id) DO NOTHING
	`, address2blocksTable))
	if err != nil {
		return err
	}

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

	// Prepare statement for blocks table within the transaction
	stmt, err := tx.Prepare(template)
	if err != nil {
		return fmt.Errorf("error preparing statement for blocks table: %w", err)
	}
	defer stmt.Close()

	// Prepare statement for address2blocks table within the transaction
	stmt2, err := tx.Prepare(template2)
	if err != nil {
		return fmt.Errorf("error preparing statement for address2blocks table: %w", err)
	}
	defer stmt2.Close()

	// Insert items
	for _, item := range items {
		ts, err := extractTimestamp(item.Extrinsics)
		if err != nil {
			log.Printf("warning: blockID %s could not find timestamp %v", item.ID, err)
		}
		// log.Printf("Inserting item %s at %s", item.ID, ts)
		_, err = stmt.Exec(
			item.ID,
			ts,
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
			log.Printf("warning: error extracting addresses from extrinsics: %v", err)
			continue
		}

		// Insert into address2blocks table
		for _, address := range addresses {
			_, err = stmt2.Exec(address, item.ID)
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
	blocksTable := fmt.Sprintf("%s.blocks_%s_%s", schemaName, strings.ToLower(config.Relaychain), chainName)

	// Query for existing blocks
	template, err := s.getQueryTemplate(blocksTable, fmt.Sprintf("SELECT block_id FROM %s WHERE block_id BETWEEN $1 AND $2", blocksTable))
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(template, startRange, endRange)
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
