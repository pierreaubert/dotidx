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
	for i := range config.MaxWorkers / 2 {
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
		for _, b:= range existingBlocks {
			if b {
				known += 1
			}
		}
		toProcess := 1+endRange-startRange-known
		if toProcess > 0 {
			log.Printf(
				"Processing %d blocks in range %d-%d blocks (full range %d-%d) %4.1f%% done!",
				1+endRange-startRange-known,
				startRange, endRange, config.StartRange, config.EndRange,
				float64((startRange-config.StartRange)/(1+config.EndRange-config.StartRange)),
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

	// Close the channels to signal that no more blocks will be sent
	close(blockCh)
	close(batchCh)

	// Wait for all workers to finish
	if !config.Live {
		wg.Wait()
	}
}

// processBlockBatch fetches and processes a batch of blocks using fetchBlockRange
func processBlockBatch(ctx context.Context, blockIDs []int, config Config, db Database, reader ChainReader, workerID int) {
	if len(blockIDs) == 0 {
		return
	}

	// log.Printf("Worker %d processing batch of %d blocks from %d to %d", workerID, len(blockIDs), blockIDs[0], blockIDs[len(blockIDs)-1])

	// Fetch the blocks using fetchBlockRange
	blocks, err := reader.FetchBlockRange(ctx, blockIDs)
	if err != nil {
		log.Printf("Worker %d error fetching blocks %v: %v", workerID, blockIDs, err)
		return
	}

	// Save the blocks to the database
	if err := db.Save(blocks, config); err != nil {
		log.Printf("Worker %d error saving blocks to database: %v", workerID, err)
		return
	}

}

// processSingleBlock fetches and processes a single block using fetchBlock
func processSingleBlock(ctx context.Context, blockID int, config Config, db Database, reader ChainReader, workerID int) {
	log.Printf("Worker %d processing single block %d", workerID, blockID)

	// Fetch the block data using fetchBlock
	block, err := reader.FetchBlock(ctx, blockID)
	if err != nil {
		log.Printf("Worker %d error fetching block %d: %v", workerID, blockID, err)
		return
	}

	// Save the block to the database
	if err := db.Save([]BlockData{block}, config); err != nil {
		// log.Printf("Worker %d error saving block %d to database: %v", workerID, blockID, err)
		return
	}

}

// MonitorNewBlocks continuously monitors for new blocks and adds them to the database
func monitorNewBlocks(ctx context.Context, config Config, db Database, reader ChainReader, lastProcessedBlock int) error {
	// Create a ticker that ticks every second
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Block monitor stopped due to context cancellation")
			return nil
		case <-ticker.C:
			// Fetch the current head block
			headBlock, err := reader.GetChainHeadID()
			if err != nil {
				log.Printf("Error fetching head block: %v", err)
				continue
			}

			if headBlock > lastProcessedBlock {
				log.Printf("New blocks detected: %d to %d", lastProcessedBlock+1, headBlock)

				// Create array of block IDs to fetch
				blockIDs := make([]int, 0, headBlock-lastProcessedBlock)
				for id := lastProcessedBlock + 1; id <= headBlock; id++ {
					blockIDs = append(blockIDs, id)
				}

				// Fetch and process the new blocks
				blocks, err := reader.FetchBlockRange(ctx, blockIDs)
				if err != nil {
					log.Printf("Error fetching block range: %v", err)
					continue
				}

				// Save the blocks to the database
				if err := db.Save(blocks, config); err != nil {
					log.Printf("Error saving blocks to database: %v", err)
					continue
				}

				// Update the last processed block
				lastProcessedBlock = headBlock
			}
		}
	}
}

type Stats struct {
	db           Database
	reader       ChainReader
	tickerHeader *time.Ticker
	tickerInfo   *time.Ticker
	context      context.Context
}

func NewStats(ctx context.Context, db Database, reader ChainReader) *Stats {
	return &Stats{
		db:           db,
		reader:       reader,
		tickerHeader: time.NewTicker(300 * time.Second),
		tickerInfo:   time.NewTicker(15 * time.Second),
		context:      ctx,
	}
}

func (s *Stats) Print() error {
	for {
		select {
		case <-s.context.Done():
			return nil
		case <-s.tickerHeader.C:
			log.Printf("+-- Blocks -------------|------ Chain Reader --|------- DBwriter -------------+")
			log.Printf("| #----#  b/s  b/s  b/s | Latency (ms)   Error |  tr/s   Latency (ms)   Error |")
			log.Printf("|          1d   1h   5m | min  avg  max      %% |         min  avg  max     %%  |")
			log.Printf("+-----------------------|----------------------|------------------------------|")
		case <-s.tickerInfo.C:
			rs := s.reader.GetStats().bucketsStats
			ds := s.db.GetStats().bucketsStats
			rs_rate := float64(rs[0].failures) / float64(rs[0].count+rs[0].failures) * 100
			ds_rate := float64(ds[0].failures) / float64(ds[0].count+ds[0].failures) * 100
			log.Printf("| %6d %4.1f %4.1f %4.1f | %4d %4d %5d %3.0f%% | %6.1f  %4d %4d %5d %3.0f%% |",
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
}
