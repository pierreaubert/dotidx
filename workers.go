package main

import (
	"context"
	"log"
	"sync"
	"time"
)

func startWorkers(
	ctx context.Context,
	config Config,
	db Database,
	reader ChainReader,
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
					processSingleBlock(ctx, blockID, config, db, reader, workerID)
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
					processBlockBatch(ctx, blockIDs, config, db, reader, workerID)
				}
			}
		}(i)
	}

	// Get existing blocks from the database, limited to 100k in one go
	const stepRange = 100000
	startRange := config.StartRange
	endRange := min(config.StartRange+stepRange, config.EndRange)

	for endRange <= config.EndRange {

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
		toProcess := 1 + endRange - startRange - known
		if toProcess > 0 {
			log.Printf(
				"Processing %d blocks in range %d-%d blocks (full range %d-%d) %4.1f%% done!",
				1+endRange-startRange-known,
				startRange, endRange, config.StartRange, config.EndRange,
				float64((startRange-config.StartRange)/(1+config.EndRange-config.StartRange)*100),
			)
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

// ProcessSingleBlock fetches and processes a single block using fetchBlock
func processSingleBlock(ctx context.Context, blockID int, config Config, db Database, reader ChainReader, workerID int) {
	block, err := reader.FetchBlock(ctx, blockID)
	if err != nil {
		log.Printf("Error fetching block %d: %v", blockID, err)
		return
	}

	// Save block to database
	err = db.Save([]BlockData{block}, config)
	if err != nil {
		log.Printf("Error saving block %d: %v", blockID, err)
		return
	}
}

// ProcessBlockBatch fetches and processes a batch of blocks using fetchBlockRange
func processBlockBatch(ctx context.Context, blockIDs []int, config Config, db Database, reader ChainReader, workerID int) {
	if len(blockIDs) == 0 {
		return
	}

	// Create the array of block IDs from the range
	ids := make([]int, 0, blockIDs[len(blockIDs)-1] - blockIDs[0] + 1)
	for i := blockIDs[0]; i <= blockIDs[len(blockIDs)-1]; i++ {
		ids = append(ids, i)
	}

	blockRange, err := reader.FetchBlockRange(ctx, ids)
	if err != nil {
		log.Printf("Error fetching blocks %d-%d: %v", blockIDs[0], blockIDs[len(blockIDs)-1], err)
		return
	}

	if len(blockRange) == 0 {
		log.Printf("No blocks returned for range %d-%d", blockIDs[0], blockIDs[len(blockIDs)-1])
		return
	}

	// Save blocks to database
	err = db.Save(blockRange, config)
	if err != nil {
		log.Printf("Error saving blocks %d-%d: %v", blockIDs[0], blockIDs[len(blockIDs)-1], err)
		return
	}
}

// MonitorNewBlocks continuously monitors for new blocks and adds them to the database
func monitorNewBlocks(ctx context.Context, config Config, db Database, reader ChainReader, lastProcessedBlock int) error {
	log.Printf("Starting to monitor new blocks from block %d", lastProcessedBlock)

	nextBlockID := lastProcessedBlock + 1
	currentHeadID := lastProcessedBlock

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Fetch the current head block
			headID, err := reader.GetChainHeadID()
			if err != nil {
				log.Printf("Error fetching head block: %v", err)
				continue
			}

			// Log progress if head has moved
			if headID > currentHeadID {
				// log.Printf("Current head is at block %d, next to process is block %d", headID, nextBlockID)
				currentHeadID = headID
			}

			// Process blocks from nextBlockID up to the head
			for nextBlockID <= headID {
				block, err := reader.FetchBlock(ctx, nextBlockID)
				if err != nil {
					log.Printf("Error fetching block %d: %v", nextBlockID, err)
					break
				}

				// Save block to database
				err = db.Save([]BlockData{block}, config)
				if err != nil {
					log.Printf("Error saving block %d: %v", nextBlockID, err)
					break
				}

				// log.Printf("Processed block %d", nextBlockID)
				nextBlockID++
			}
		}
	}
}

// Stats struct to track and print statistics
type Stats struct {
	db           Database
	reader       ChainReader
	tickerHeader *time.Ticker
	tickerInfo   *time.Ticker
	context      context.Context
}

// NewStats creates a new Stats instance
func NewStats(ctx context.Context, db Database, reader ChainReader) *Stats {
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
	log.Printf("+--- Blocks ---------------|------ Chain Reader ----|------- DBwriter ---------------+")
	log.Printf("| #-----#  b/s   b/s   b/s | Latency (ms)     Error |  tr/s   Latency (ms)     Error |")
	log.Printf("|          1d    1h    5m  | min  avg  max        %% |         min  avg  max       %%  |")
	log.Printf("+--------------------------|------------------------|--------------------------------|")
}

func (s *Stats) printStats(stats *MetricsStats) {
	if stats == nil {
		return
	}

	rs := stats.bucketsStats
	ds := stats.bucketsStats

	if len(rs) > 0 && len(ds) > 0 {
		rs_rate := float64(0)
		ds_rate := float64(0)

		if rs[0].count+rs[0].failures > 0 {
			rs_rate = float64(rs[0].failures) / float64(rs[0].count+rs[0].failures) * 100
		}
		if ds[0].count+ds[0].failures > 0 {
			ds_rate = float64(ds[0].failures) / float64(ds[0].count+ds[0].failures) * 100
		}

		log.Printf("| %7d %5.1f %5.1f %5.1f | %4d %4d %5d %5.0f%% | %6.1f  %4d %4d %5d %5.0f%% |",
			rs[0].count, rs[0].rate, rs[1].rate, rs[2].rate,
			rs[0].min.Milliseconds(),
			rs[0].avg.Milliseconds(),
			rs[0].max.Milliseconds(),
			rs_rate,
			ds[0].rate,
			ds[0].min.Milliseconds(),
			ds[0].avg.Milliseconds(),
			ds[0].max.Milliseconds(),
			ds_rate)
	}
}
