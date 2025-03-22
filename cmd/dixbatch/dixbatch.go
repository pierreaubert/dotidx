package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"

	"github.com/pierreaubert/dotidx"
)

func validateConfig(config dotidx.Config) error {

	if config.ChainReaderURL == "" {
		return fmt.Errorf("chainReader url is required")
	}

	if config.DatabaseURL == "" {
		return fmt.Errorf("database url is required")
	}

	if config.BatchSize <= 0 {
		return fmt.Errorf("batch size must be greater than 0")
	}

	if config.MaxWorkers <= 0 {
		return fmt.Errorf("max workers must be greater than 0")
	}

	if config.Chain == "" {
		return fmt.Errorf("chain name is required")
	}

	return nil
}


func main() {
	// Parse command line arguments
	config := dotidx.ParseFlags()

	// Set up logging
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Validate configuration
	if err := validateConfig(config); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}
	log.Printf("Relay chain: %s chain: %s", config.Relaychain, config.Chain)

	// ----------------------------------------------------------------------
	// ChainReader
	// ----------------------------------------------------------------------
	reader := dotidx.NewSidecar(config.ChainReaderURL)
	// Test the sidecar service
	if err := reader.Ping(); err != nil {
		log.Fatalf("Sidecar service test failed: %v", err)
	}
	log.Println("Successfully connected to Sidecar service")

	headBlockID, err := reader.GetChainHeadID()
	if err != nil {
		log.Fatalf("Failed to fetch head block: %v", err)
	}
	log.Printf("Current head block is %d", headBlockID)

	// ----------------------------------------------------------------------
	// Set up context with cancellation for graceful shutdown
	// ----------------------------------------------------------------------
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	dotidx.SetupSignalHandler(cancel)

	// ----------------------------------------------------------------------
	// Database
	// ----------------------------------------------------------------------
	var db *sql.DB
	if strings.Contains(config.DatabaseURL, "postgres") {
		// Ensure sslmode=disable is in the PostgreSQL URI if not already present
		if !strings.Contains(config.DatabaseURL, "sslmode=") {
			if strings.Contains(config.DatabaseURL, "?") {
				config.DatabaseURL += "&sslmode=disable"
			} else {
				config.DatabaseURL += "?sslmode=disable"
			}
		}

		// Create database connection
		var err error
		db, err = sql.Open("postgres", config.DatabaseURL)
		if err != nil {
			log.Fatalf("Error opening database: %v", err)
		}
		defer db.Close()
	} else {
		log.Fatalf("unsupported database: %s", config.DatabaseURL)
	}

	// Create database instance
	database := dotidx.NewSQLDatabase(db, ctx)

	// Create tables
	firstBlock, err := reader.FetchBlock(ctx, 1)
	if err != nil {
		log.Fatalf("Cannot get block 1: %v", err)
	}
	firstTimestamp, err := dotidx.ExtractTimestamp(firstBlock.Extrinsics)
	if err != nil {
		// some parachain do not have the pallet timestamp
		firstTimestamp = ""
	}
	lastBlock, err := reader.FetchBlock(ctx, headBlockID)
	if err != nil {
		log.Fatalf("Cannot get head block %d: %v", headBlockID, err)
	}
	lastTimestamp, err := dotidx.ExtractTimestamp(lastBlock.Extrinsics)
	if err != nil {
		lastTimestamp = time.Now().Format("2006-01-02 15:04:05")
	}

	if err := database.CreateTable(config, firstTimestamp, lastTimestamp); err != nil {
		log.Fatalf("Error creating tables: %v", err)
	}

	// Test the connection
	if err := database.Ping(); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}

	log.Printf("Successfully connected to database %s", config.DatabaseURL)

	// print some stats
	go func() {
		if err := NewStats(ctx, database, reader).Print(); err != nil {
			log.Fatalf("Error monitoring stats: %v", err)
		}
	}()

	startWorkers(ctx, config, database, reader, headBlockID)

	log.Println("All tasks completed")
}

func startWorkers(
	ctx context.Context,
	config dotidx.Config,
	db dotidx.Database,
	reader dotidx.ChainReader,
	headID int) {

	config.EndRange = min(config.EndRange, headID)

	log.Printf("Starting %d workers to process blocks %d to %d head is at %d",
		config.MaxWorkers, config.StartRange, config.EndRange, headID)

	// Create a channel for block IDs
	blockCh := make(chan int, config.BatchSize)

	// Create a channel for batch processing
	batchCh := make(chan []int, config.MaxWorkers)

	// Create a wait group to wait for all workers to finish
	var wg sync.WaitGroup

	// Start single block workers
	for i := 0; i < config.MaxWorkers/2; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			// log.Printf("Single block worker %d started", workerID)

			for {
				select {
				case <-ctx.Done():
					return
				case blockID, ok := <-blockCh:
					if !ok {
						return
					}

					// Process a single block
					dotidx.ProcessSingleBlock(ctx, blockID, config, db, reader)
				}
			}
		}(i)
	}

	// Start batch workers
	for i := config.MaxWorkers / 2; i < config.MaxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				case blockIDs, ok := <-batchCh:
					if !ok {
						return
					}

					// Process a batch of blocks
					dotidx.ProcessBlockBatch(ctx, blockIDs, config, db, reader)
				}
			}
		}(i)
	}

	// Get existing blocks from the database, limited to 100k in one go
	const stepRange = 100000
	startRange := config.StartRange
	endRange := min(config.StartRange+stepRange, config.EndRange)

	for startRange <= config.EndRange {

		// Collect blocks to process, identifying continuous ranges for batch processing
		var currentBatch []int
		var lastBlockID = -1

		existingBlocks, err := db.GetExistingBlocks(startRange, endRange, config)
		if err != nil {
			log.Printf("Error getting existing blocks: %v", err)
			// Continue with empty map if there was an error
			existingBlocks = make(map[int]bool)
		}

		known := 0
		for _, b := range existingBlocks {
			if b {
				known += 1
			}
		}

		// Send block IDs to the appropriate channel, skipping ones that already exist
		for blockID := startRange; blockID <= endRange; blockID++ {
			if existingBlocks[blockID] {
				// If we have a batch in progress, send it since we're skipping this block
				if len(currentBatch) > 0 {
					select {
					case <-ctx.Done():
						log.Println("Block sender stopped due to context cancellation")
						close(blockCh)
						close(batchCh)
						return
					case batchCh <- currentBatch:
						// Batch sent to channel
						currentBatch = nil
					}
				}

				lastBlockID = -1 // Reset the sequence
				continue
			}

			// Check if this block is continuous with the previous one
			if lastBlockID != -1 && blockID == lastBlockID+1 {
				// Add to the current batch
				currentBatch = append(currentBatch, blockID)
			} else {
				// If we have a batch in progress, send it
				if len(currentBatch) > 0 {
					select {
					case <-ctx.Done():
						log.Println("Block sender stopped due to context cancellation")
						close(blockCh)
						close(batchCh)
						return
					case batchCh <- currentBatch:
						// Batch sent to channel
					}
				}

				// Start a new batch with this block
				currentBatch = []int{blockID}
			}

			lastBlockID = blockID

			// If the batch is large enough, send it
			if len(currentBatch) >= config.BatchSize {
				select {
				case <-ctx.Done():
					log.Println("Block sender stopped due to context cancellation")
					close(blockCh)
					close(batchCh)
					return
				case batchCh <- currentBatch:
					// Batch sent to channel
					currentBatch = nil
					lastBlockID = -1 // Reset the sequence
				}
			}

		}

		// Send any remaining batch
		if len(currentBatch) > 0 {
			select {
			case <-ctx.Done():
				log.Println("Block sender stopped due to context cancellation")
				close(blockCh)
				close(batchCh)
				return
			case batchCh <- currentBatch:
				// Batch sent to channel
			}
		}

		startRange = endRange
		if startRange >= config.EndRange {
			break
		}
		endRange = min(endRange+stepRange, config.EndRange)
	}

	close(blockCh)
	close(batchCh)

	wg.Wait()
}

// Stats struct to track and print statistics
type Stats struct {
	db           dotidx.Database
	reader       dotidx.ChainReader
	tickerHeader *time.Ticker
	tickerInfo   *time.Ticker
	context      context.Context
}

// NewStats creates a new Stats instance
func NewStats(ctx context.Context, db dotidx.Database, reader dotidx.ChainReader) *Stats {
	return &Stats{
		db:           db,
		reader:       reader,
		tickerHeader: time.NewTicker(5 * time.Minute),
		tickerInfo:   time.NewTicker(15 * time.Second),
		context:      ctx,
	}
}

// Print prints statistics
func (s *Stats) Print() error {
	for {
		select {
		case <-s.context.Done():
			return s.context.Err()
		case <-s.tickerHeader.C:
			s.printHeader()
		case <-s.tickerInfo.C:
			stats := s.db.GetStats()
			s.printStats(stats)
		}
	}
}

// printStats prints the database statistics
func (s *Stats) printHeader() {
	log.Printf("+--- Blocks ----------------|------ Chain Reader ----|------- DBwriter ---------------+")
	log.Printf("| #-----#  b/s   b/s   b/s  | Latency (ms)     Error |  tr/s   Latency (ms)     Error |")
	log.Printf("|           1d    1h    5m  | min  avg  max        %% |         min  avg  max       %%  |")
	log.Printf("+---------------------------|------------------------|--------------------------------|")
}

func (s *Stats) printStats(stats *dotidx.MetricsStats) {
	if stats == nil {
		return
	}

	rs := stats.BucketsStats
	ds := stats.BucketsStats

	if len(rs) > 0 && len(ds) > 0 {
		rs_rate := rs[0].RateSinceStart()
		ds_rate := ds[0].RateSinceStart()
		log.Printf("| %7d %5.1f %5.1f %5.1f | %4d %4d %5d %5.0f%% | %6.1f  %4d %4d %5d %5.0f%% |",
			rs[0].Count, rs[0].Rate, rs[1].Rate, rs[2].Rate,
			rs[0].Min.Milliseconds(),
			rs[0].Avg.Milliseconds(),
			rs[0].Max.Milliseconds(),
			rs_rate,
			ds[0].Rate,
			ds[0].Min.Milliseconds(),
			ds[0].Avg.Milliseconds(),
			ds[0].Max.Milliseconds(),
			ds_rate)
	}
}
