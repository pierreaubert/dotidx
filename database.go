package dotidx

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

type DatabaseInfo struct {
	Relaychain string
	Chain      string
}

// Database defines the interface for database operations
type Database interface {
	CreateTable(relayChain, chain, firstTimestamp, lastTimestamp string) error
	CreateIndex(relayChain, chain string) error
	Save(items []BlockData, relayChain, chain string) error
	GetExistingBlocks(relayChain, chain string, startRange, endRange int) (map[int]bool, error)
	Ping() error
	GetStats() *MetricsStats
	DoUpgrade(relayChain, chain string) error
	Close() error
	UpdateMaterializedTables(relayChain, chain string) error
	GetDatabaseInfo() ([]DatabaseInfo, error)
}

// DBPoolConfig contains the configuration for the database connection pool
type DBPoolConfig struct {
	MaxOpenConns    int           // Maximum number of open connections
	MaxIdleConns    int           // Maximum number of idle connections
	ConnMaxLifetime time.Duration // Maximum lifetime of a connection
	ConnMaxIdleTime time.Duration // Maximum idle time of a connection
}

const schemaName = "chain"
const fastTablespaceRoot = "fast"
const fastTablespaceNumber = 4
const slowTablespaceRoot = "slow"
const slowTablespaceNumber = 6

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

	// continous queries
	materializedTicker *time.Ticker
}

// NewSQLDatabase creates a new Database instance
func NewSQLDatabaseWithDB(db *sql.DB) *SQLDatabase {
	return NewSQLDatabaseWithPool(
		db,
		DefaultDBPoolConfig())
}

// NewSQLDatabase creates a new Database instance
func NewSQLDatabase(config MgrConfig) *SQLDatabase {
	databaseURL := DBUrl(config)
	if strings.Contains(databaseURL, "postgres") {
		db, err := sql.Open("postgres", databaseURL)
		if err != nil {
			log.Fatalf("Error opening database: %v", err)
		}
		return NewSQLDatabaseWithPool(db, DefaultDBPoolConfig())
	}

	log.Fatalf("unsupported database: %s", databaseURL)
	return nil
}

// NewSQLDatabaseWithPool creates a new Database instance with custom connection pool settings
func NewSQLDatabaseWithPool(db *sql.DB, poolCfg DBPoolConfig) *SQLDatabase {
	// Configure connection pool
	db.SetMaxOpenConns(poolCfg.MaxOpenConns)
	db.SetMaxIdleConns(poolCfg.MaxIdleConns)
	db.SetConnMaxLifetime(poolCfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(poolCfg.ConnMaxIdleTime)

	return &SQLDatabase{
		db:                 db,
		materializedTicker: time.NewTicker(15 * time.Minute),
		metrics:            NewMetrics("Postgres"),
		poolCfg:            poolCfg,
	}
}

// Close closes the database connection pool
func (s *SQLDatabase) Close() error {
	return s.db.Close()
}

func (s *SQLDatabase) DoUpgrade(relayChain, chain string) error {

	// create dotidx version table to track migrations
	_, err := s.db.Exec(`
    CREATE TABLE IF NOT EXISTS dotidx_version (
	version_id INTEGER NOT NULL,
	timestamp TIMESTAMP(4) WITHOUT TIME ZONE,
        CONSTRAINT dotidx_version_pkey PRIMARY KEY (version_id)
    )
        `)
	if err != nil {
		return fmt.Errorf("error creating table: %w", err)
	}

	return nil
}

// sanitizeChainName removes non-alphanumeric characters and the relaychain name from the chain name
func sanitizeChainName(initialRelaychainName, initialChainName string) string {
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
	if initialChainName != initialRelaychainName && chainName != relaychainName {
		chainName = strings.ReplaceAll(chainName, relaychainName, "")
	}
	return chainName
}

func GetBlocksTableName(relayChain, chain string) (name string) {
	chainName := sanitizeChainName(relayChain, chain)
	name = fmt.Sprintf("%s.blocks_%s_%s", schemaName, strings.ToLower(relayChain), chainName)
	return
}

func GetBlocksPrimaryKeyName(relayChain, chain string) (name string) {
	chainName := sanitizeChainName(relayChain, chain)
	name = fmt.Sprintf("blocks_%s_%s", strings.ToLower(relayChain), chainName)
	return
}

func GetAddressTableName(relayChain, chain string) (name string) {
	chainName := sanitizeChainName(relayChain, chain)
	name = fmt.Sprintf("%s.address2blocks_%s_%s", schemaName, strings.ToLower(relayChain), chainName)
	return
}

func GetStatsPerMonthTableName(relayChain, chain string) (name string) {
	chainName := sanitizeChainName(relayChain, chain)
	name = fmt.Sprintf("%s.stats_per_month_%s_%s", schemaName, strings.ToLower(relayChain), chainName)
	return
}

func (s *SQLDatabase) CreateTableBlocks(relayChain, chain string) error {
	blocksTable := GetBlocksTableName(relayChain, chain)
	blocksPK := GetBlocksPrimaryKeyName(relayChain, chain)

	// Create the blocks table
	template := fmt.Sprintf(`
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
  CONSTRAINT      %[2]s_pk PRIMARY KEY (block_id, created_at)
) PARTITION BY RANGE (created_at);
ALTER TABLE IF EXISTS %[1]s OWNER to dotidx;
REVOKE ALL ON TABLE %[1]s FROM PUBLIC;
GRANT SELECT ON TABLE %[1]s TO PUBLIC;
GRANT ALL ON TABLE %[1]s TO dotidx;
	`, blocksTable, blocksPK)

	_, err := s.db.Exec(template)
	if err != nil {
		return fmt.Errorf("error creating blocks table: %s %w", template, err)
	}

	return nil
}

func (s *SQLDatabase) CreateTableBlocksPartitions(relayChain, chain, firstTimestamp, lastTimestamp string) error {
	blocksTable := GetBlocksTableName(relayChain, chain)

	// kusame stated oct 2019
	firstYear, firstMonth := 2019, 10
	if relayChain == "polkadot" {
		firstYear, firstMonth = 2020, 05
	}
	if firstTimestamp != "" {
		firstTime, err := time.Parse("2000-01-01 00:00:00", firstTimestamp)
		if err == nil {
			return fmt.Errorf("Parsing time failed %w", err)
		}
		_, firstMonthAsMonth, _ := firstTime.Date()
		firstMonth = int(firstMonthAsMonth) - 1
	}

	// Spread by month across the partition
	slow := 0
	fast := 0
	slowOrFast := ""
	for yearIdx := range 6 {
		year := firstYear + yearIdx
		if year >= time.Now().Year() {
			slowOrFast = fmt.Sprintf("%s%d", fastTablespaceRoot, fast)
			fast = min(fast+1, fastTablespaceNumber-1)
		} else {
			slowOrFast = fmt.Sprintf("%s%d", slowTablespaceRoot, slow)
			slow = min(slow+1, slowTablespaceNumber-1)
		}
		for month := range 12 {
			// skip tables if no data
			if year < firstYear || (year == firstYear && month < firstMonth) {
				continue
			}
			from_date := fmt.Sprintf("%04d-%02d-01 00:00:00.0000", year, month+1)
			to_date := fmt.Sprintf("%04d-%02d-01 00:00:00.0000", year, month+2)
			if month == 11 {
				to_date = fmt.Sprintf("%04d-%02d-01 00:00:00.0000", year+1, 1)
			}
			parts := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s_%04[2]d_%02[3]d PARTITION OF %[1]s
  FOR VALUES FROM (timestamp '%[5]s') TO (timestamp '%[6]s')
  TABLESPACE dotidx_%[7]s;
ALTER TABLE IF EXISTS %[1]s_%04[2]d_%02[3]d OWNER to dotidx;
REVOKE ALL ON TABLE %[1]s_%04[2]d_%02[3]d FROM PUBLIC;
GRANT SELECT ON TABLE %[1]s_%04[2]d_%02[3]d TO PUBLIC;
GRANT ALL ON TABLE %[1]s_%04[2]d_%02[3]d TO dotidx;
	`,
				blocksTable, // 1
				year,        // 2
				month+1,     // 3
				month+2,     // 4
				from_date,   // 5
				to_date,     // 6
				slowOrFast,  // 7
			)
			_, err := s.db.Exec(parts)
			if err != nil {
				log.Printf("sql %s", parts)
				return fmt.Errorf("error creating blocks partition table: %w", err)
			}
		}
	}

	return nil
}

func (s *SQLDatabase) CreateTableAddress2Blocks(relayChain, chain string) error {
	address2blocksTable := GetAddressTableName(relayChain, chain)

	template := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
     address TEXT,
     block_id INTEGER,
     PRIMARY KEY (address, block_id)
) PARTITION BY HASH(address);
ALTER TABLE IF EXISTS %[1]s OWNER to dotidx;
REVOKE ALL ON TABLE %[1]s FROM PUBLIC;
GRANT SELECT ON TABLE %[1]s TO PUBLIC;
GRANT ALL ON TABLE %[1]s TO dotidx;
	`, address2blocksTable)

	_, err := s.db.Exec(template)
	if err != nil {
		log.Printf("sql %s", template)
		return fmt.Errorf("error creating address2blocks table: %w", err)
	}

	return nil
}

func (s *SQLDatabase) CreateTableAddress2BlocksPartitions(relayChain, chain string) error {
	address2blocksTable := GetAddressTableName(relayChain, chain)

	// spread across fast disks to improve access time
	for fast := range fastTablespaceNumber {
		parts := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s_%1[2]d PARTITION OF %[1]s
  FOR VALUES WITH (modulus %[3]d, remainder %[2]d)
  TABLESPACE dotidx_fast%[2]d;
ALTER TABLE IF EXISTS %[1]s_%1[2]d OWNER to dotidx;
REVOKE ALL ON TABLE %[1]s_%1[2]d FROM PUBLIC;
GRANT SELECT ON TABLE %[1]s_%1[2]d TO PUBLIC;
GRANT ALL ON TABLE %[1]s_%1[2]d TO dotidx;
	`,
			address2blocksTable,  // 1
			fast,                 // 2
			fastTablespaceNumber, // 3
		)
		_, err := s.db.Exec(parts)
		if err != nil {
			log.Printf("sql %s", parts)
			return fmt.Errorf("error creating partitions for address2blocks: %w", err)
		}
	}

	return nil
}

func (s *SQLDatabase) CreateMaterializedTableForStats(relayChain, chain string) error {
	tableName := GetBlocksTableName(relayChain, chain)
	statsPerMonthViewName := GetStatsPerMonthTableName(relayChain, chain)
	query := fmt.Sprintf(`
		CREATE MATERIALIZED VIEW IF NOT EXISTS %s AS
                SELECT
		        date_trunc('month',created_at)::date as date,
			count(*) as total,
			min(block_id) as min_block_id,
			max(block_id) as max_block_id
		FROM %s
		GROUP BY date
		ORDER BY date DESC
                ;
	`, statsPerMonthViewName, tableName)

	_, err := s.db.Exec(query)
	if err != nil {
		log.Printf("sql %s", query)
		return fmt.Errorf("error failed to create materialized table for statistics: %w", err)
	}
	return nil
}

func (s *SQLDatabase) CreateDotidxTable(relayChain, chain string) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.dotidx (
                    relay_chain TEXT NOT NULL,
                    chain       TEXT NOT NULL,
                    CONSTRAINT dotidx_pk PRIMARY KEY (relay_chain, chain)
                );
	`, schemaName)

	if _, err := s.db.Exec(query); err != nil {
		log.Printf("sql %s", query)
		return fmt.Errorf("%w", err)
	}

	inserts := fmt.Sprintf(`
INSERT INTO %s.dotidx (relay_chain, chain)
VALUES ('%s', '%s')
ON CONFLICT (relay_chain, chain) DO NOTHING;
`,
		schemaName,
		strings.ToLower(relayChain),
		sanitizeChainName(relayChain, chain),
	)

	// log.Printf("%s", inserts)

	if _, err := s.db.Exec(inserts); err != nil {
		log.Printf("sql %s", inserts)
		return fmt.Errorf("error failed to create insert in dotidx: %w", err)
	}

	return nil
}

func (s *SQLDatabase) CreateTable(relayChain, chain, firstTimestamp, lastTimestamp string) error {

	if err := s.CreateDotidxTable(relayChain, chain); err != nil {
		return fmt.Errorf("error creating dotidx table: %w", err)
	}

	if err := s.CreateTableBlocks(relayChain, chain); err != nil {
		return fmt.Errorf("error creating table blocks: %w", err)
	}

	if err := s.CreateTableBlocksPartitions(relayChain, chain, firstTimestamp, lastTimestamp); err != nil {
		return fmt.Errorf("error creating table blocks partitions: %w", err)
	}

	if err := s.CreateTableAddress2Blocks(relayChain, chain); err != nil {
		return fmt.Errorf("error creating table address2blocks: %w", err)
	}

	if err := s.CreateTableAddress2BlocksPartitions(relayChain, chain); err != nil {
		return fmt.Errorf("error creating table address2blocks partitions: %w", err)
	}

	if err := s.CreateMaterializedTableForStats(relayChain, chain); err != nil {
		return fmt.Errorf("error creating materialized table for statistics: %w", err)
	}

	return nil
}

// TODO: adapt to the new partionning
// when tables are full (a month) they are immutable so we can write the index once and forall
func (s *SQLDatabase) CreateIndex(relayChain, chain string) error {
	blocksTable := GetBlocksTableName(relayChain, chain)

	template := fmt.Sprintf(`
                CREATE INDEX IF NOT EXISTS extrinsincs_idx
                ON %s USING gin(extrinsics jsonb_path_ops)
                WITH (fastupdate=True)
                TABLESPACE pg_default;
	`, blocksTable)
	_, err := s.db.Exec(template)
	if err != nil {
		return fmt.Errorf("error creating index on address column: %w", err)
	}

	return nil
}

func (s *SQLDatabase) Save(items []BlockData, relayChain, chain string) error {
	if len(items) == 0 {
		return nil
	}

	start := time.Now()
	defer func(start time.Time) {
		go func(start time.Time, err error) {
			s.metrics.RecordLatency(start, len(items), err)
		}(start, nil)
	}(start)

	// Get table names
	blocksTable := GetBlocksTableName(relayChain, chain)
	address2blocksTable := GetAddressTableName(relayChain, chain)

	// log.Printf("Saving batch of %d items to database", len(items))
	// log.Printf("Blocks table: %s", blocksTable)
	// log.Printf("Address2blocks table: %s", address2blocksTable)

	// Create insert query templates without using prepared statements
	blocksInsertQuery := fmt.Sprintf(
		"INSERT INTO %s ("+
			"block_id, created_at, hash, parent_hash, state_root, extrinsics_root, "+
			"author_id, finalized, on_initialize, on_finalize, logs, extrinsics"+
			") VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) "+
			"ON CONFLICT (block_id, created_at) DO UPDATE SET "+
			"created_at = EXCLUDED.created_at, "+
			"hash = EXCLUDED.hash, "+
			"parent_hash = EXCLUDED.parent_hash, "+
			"state_root = EXCLUDED.state_root, "+
			"extrinsics_root = EXCLUDED.extrinsics_root, "+
			"author_id = EXCLUDED.author_id, "+
			"finalized = EXCLUDED.finalized, "+
			"on_initialize = EXCLUDED.on_initialize, "+
			"on_finalize = EXCLUDED.on_finalize, "+
			"logs = EXCLUDED.logs, "+
			"extrinsics = EXCLUDED.extrinsics",
		blocksTable)

	addressInsertQuery := fmt.Sprintf(
		"INSERT INTO %s (address, block_id) VALUES ($1, $2) "+
			"ON CONFLICT (address, block_id) DO NOTHING",
		address2blocksTable)

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

	// Insert items directly without using prepared statements
	for _, item := range items {
		ts, err := ExtractTimestamp(item.Extrinsics)
		if err != nil {
			// log.Printf("warning: blockID %s could not find timestamp %v", item.ID, err)
			// faking it
			id, _ := strconv.ParseInt(item.ID, 10, 32)
			milli := id % 1000
			sec := (id / 1000) % 60
			min := (id / 60000) % 60
			hour := (id / 3600000) % 60
			ts = fmt.Sprintf("2000-01-01 %02d:%02d:%02d.%04d", hour, min, sec, milli)
		}

		// Insert into blocks table using direct execution
		_, err = tx.Exec(
			blocksInsertQuery,
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
			_, err = tx.Exec(addressInsertQuery, address, item.ID)
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
func (s *SQLDatabase) GetExistingBlocks(relayChain, chain string, startRange, endRange int) (map[int]bool, error) {
	// Create blocks table name
	blocksTable := GetBlocksTableName(relayChain, chain)

	// Query for existing blocks - explicitly create a simple query without multiple statements
	query := fmt.Sprintf("SELECT block_id FROM %s WHERE block_id BETWEEN $1 AND $2", blocksTable)

	// Execute the query directly without using getQueryTemplate to avoid potential issues with multiple statements
	rows, err := s.db.Query(query, startRange, endRange)
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over blocks: %w", err)
	}

	return existingBlocks, nil
}

func (s *SQLDatabase) Ping() error {
	return s.db.Ping()
}

func (s *SQLDatabase) GetStats() *MetricsStats {
	return s.metrics.GetStats()
}

func (s *SQLDatabase) UpdateMaterializedTables(relayChain, chain string) error {

	template := fmt.Sprintf(`REFRESH MATERIALIZED VIEW %s;`,
		GetStatsPerMonthTableName(relayChain, chain))

	_, err := s.db.Exec(template)
	if err != nil {
		return fmt.Errorf("error creating blocks table: %s %w", template, err)
	}

	return nil
}

func (s *SQLDatabase) GetDatabaseInfo() ([]DatabaseInfo, error) {
	infos := make([]DatabaseInfo, 0)
	rows, err := s.db.Query(
		fmt.Sprintf(
			`SELECT relay_chain as relaychain, chain from %s.dotidx;`,
			schemaName))
	if err != nil {
		return nil, fmt.Errorf("Cannot get dotidx information from database: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var info DatabaseInfo
		if err = rows.Scan(&info.Relaychain, &info.Chain); err != nil {
			return nil, fmt.Errorf("Cannot scan dotidx rows")
		}
		infos = append(infos, info)
	}
	return infos, nil
}
