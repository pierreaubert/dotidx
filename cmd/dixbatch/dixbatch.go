package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	_ "github.com/lib/pq"

	dix "github.com/pierreaubert/dotidx"
)

func main() {
	configFile := flag.String("conf", "", "toml configuration file")
	chain := flag.String("chain", "", "chain")
	relayChain := flag.String("relayChain", "polkadot", "relay chain")
	flag.Parse()

	if chain == nil || *chain == "" {
		log.Fatal("Chain must be specified")
	}

	if configFile == nil || *configFile == "" {
		log.Fatal("Configuration file must be specified")
	}

	config, err := dix.LoadMgrConfig(*configFile)
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Set up logging
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	log.Printf("Starting block ingestion for %s:%s", *relayChain, *chain)

	// ----------------------------------------------------------------------
	// ChainReader
	// ----------------------------------------------------------------------
	chainReaderURL := fmt.Sprintf(`http://%s:%d`,
		config.Parachains[*relayChain][*chain].ChainreaderIP,
		config.Parachains[*relayChain][*chain].ChainreaderPort,
	)
	reader := dix.NewSidecar(chainReaderURL)
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

	if config.DotidxBatch.EndRange == -1 && headBlockID == 0 {
		log.Fatal("Cannot get head block and EndRange is not set")
	}

	if config.DotidxBatch.EndRange == -1 {
		config.DotidxBatch.EndRange = headBlockID
	}

	if headBlockID == 0 {
		headBlockID = config.DotidxBatch.EndRange
	}

	// ----------------------------------------------------------------------
	// Set up context with cancellation for graceful shutdown
	// ----------------------------------------------------------------------
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	dix.SetupSignalHandler(cancel)

	// ----------------------------------------------------------------------
	// Database
	// ----------------------------------------------------------------------
	database := dix.NewSQLDatabase(*config)

	// Test the connection
	if err := database.Ping(); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}

	log.Printf("Successfully connected to database %s", dix.DBUrlSecure(*config))

	// Create tables
	firstBlock, err := reader.FetchBlock(ctx, 1)
	if err != nil {
		log.Fatalf("Cannot get block 1: %v", err)
	}
	firstTimestamp, err := dix.ExtractTimestamp(firstBlock.Extrinsics)
	if err != nil {
		// some parachain do not have the pallet timestamp
		firstTimestamp = ""
	}
	lastBlock, err := reader.FetchBlock(ctx, headBlockID)
	if err != nil {
		log.Fatalf("Cannot get head block %d: %v", headBlockID, err)
	}
	lastTimestamp, err := dix.ExtractTimestamp(lastBlock.Extrinsics)
	if err != nil {
		lastTimestamp = time.Now().Format("2006-01-02 15:04:05")
	}

	if err := database.CreateTable(*relayChain, *chain, firstTimestamp, lastTimestamp); err != nil {
		log.Fatalf("Error creating tables: %v", err)
	}

	// print some stats
	go func() {
		if err := NewStats(ctx, database, reader).Print(); err != nil {
			log.Fatalf("Error monitoring stats: %v", err)
		}
	}()

	startWorkers(*relayChain, *chain, ctx, *config, database, reader, headBlockID)

	log.Println("All tasks completed")
}

func startWorkers(
	relayChain, chain string,
	ctx context.Context,
	config dix.MgrConfig,
	db dix.Database,
	reader dix.ChainReader,
	headID int) {

	config.DotidxBatch.EndRange = min(config.DotidxBatch.EndRange, headID)

	log.Printf("Starting %d workers to process blocks %d to %d head is at %d",
		config.DotidxBatch.MaxWorkers, config.DotidxBatch.StartRange, config.DotidxBatch.EndRange, headID)

	// Create a channel for block IDs
	blockCh := make(chan int, config.DotidxBatch.BatchSize)

	// Create a channel for batch processing
	batchCh := make(chan []int, config.DotidxBatch.MaxWorkers)

	// Create a wait group to wait for all workers to finish
	var wg sync.WaitGroup

	// Start single block workers
	for i := 0; i < config.DotidxBatch.MaxWorkers/2; i++ {
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
					dix.ProcessSingleBlock(
						ctx,
						blockID,
						relayChain,
						chain,
						db,
						reader,
					)
				}
			}
		}(i)
	}

	// Start batch workers
	for i := config.DotidxBatch.MaxWorkers / 2; i < config.DotidxBatch.MaxWorkers; i++ {
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
					dix.ProcessBlockBatch(
						ctx,
						blockIDs,
						relayChain,
						chain,
						db, reader,
					)
				}
			}
		}(i)
	}

	// Get existing blocks from the database, limited to 100k in one go
	const stepRange = 100000
	startRange := config.DotidxBatch.StartRange
	endRange := min(config.DotidxBatch.StartRange+stepRange, config.DotidxBatch.EndRange)

	for startRange <= config.DotidxBatch.EndRange {

		// Collect blocks to process, identifying continuous ranges for batch processing
		var currentBatch []int
		var lastBlockID = -1

		existingBlocks, err := db.GetExistingBlocks(
			relayChain,
			chain,
			startRange,
			endRange,
		)
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
			if len(currentBatch) >= config.DotidxBatch.BatchSize {
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
		if startRange >= config.DotidxBatch.EndRange {
			break
		}
		endRange = min(endRange+stepRange, config.DotidxBatch.EndRange)
	}

	close(blockCh)
	close(batchCh)

	wg.Wait()
}

// Stats struct to track and print statistics
type Stats struct {
	db           dix.Database
	reader       dix.ChainReader
	tickerHeader *time.Ticker
	tickerInfo   *time.Ticker
	context      context.Context
}

// NewStats creates a new Stats instance
func NewStats(ctx context.Context, db dix.Database, reader dix.ChainReader) *Stats {
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

func (s *Stats) printStats(stats *dix.MetricsStats) {
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
