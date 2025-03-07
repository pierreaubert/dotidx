package dotidx

import (
	"context"
	"database/sql"
	"log"
	"sync"
)

// startWorkers starts multiple worker goroutines to process blocks in parallel
func startWorkers(ctx context.Context, config Config, db *sql.DB) {
	log.Printf("Starting %d workers to process blocks %d to %d", config.MaxWorkers, config.StartRange, config.EndRange)

	// Get existing blocks from the database
	existingBlocks, err := getExistingBlocks(db, config.StartRange, config.EndRange, config)
	if err != nil {
		log.Printf("Error getting existing blocks: %v", err)
		// Continue with empty map if there was an error
		existingBlocks = make(map[int]bool)
	}

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
			log.Printf("Single block worker %d started", workerID)

			for {
				select {
				case <-ctx.Done():
					log.Printf("Worker %d stopped due to context cancellation", workerID)
					return
				case blockID, ok := <-blockCh:
					if !ok {
						log.Printf("Worker %d finished", workerID)
						return
					}

					// Process a single block
					processSingleBlock(ctx, blockID, config, db, workerID)
				}
			}
		}(i)
	}

	// Start batch workers
	for i := config.MaxWorkers / 2; i < config.MaxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			log.Printf("Batch worker %d started", workerID)

			for {
				select {
				case <-ctx.Done():
					log.Printf("Batch worker %d stopped due to context cancellation", workerID)
					return
				case blockIDs, ok := <-batchCh:
					if !ok {
						log.Printf("Batch worker %d finished", workerID)
						return
					}

					// Process a batch of blocks
					processBlockBatch(ctx, blockIDs, config, db, workerID)
				}
			}
		}(i)
	}

	// Collect blocks to process, identifying continuous ranges for batch processing
	var currentBatch []int
	var lastBlockID = -1

	// Send block IDs to the appropriate channel, skipping ones that already exist
	for blockID := config.StartRange; blockID <= config.EndRange; blockID++ {
		// Skip blocks that already exist in the database
		if existingBlocks[blockID] {
			log.Printf("Skipping block %d as it already exists in the database", blockID)

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

	// Close the channels to signal that no more blocks will be sent
	close(blockCh)
	close(batchCh)

	// Wait for all workers to finish
	if !config.Live {
		wg.Wait()
	}
}

// processBlockBatch fetches and processes a batch of blocks using fetchBlockRange
func processBlockBatch(ctx context.Context, blockIDs []int, config Config, db *sql.DB, workerID int) {
	if len(blockIDs) == 0 {
		return
	}

	log.Printf("Worker %d processing batch of %d blocks from %d to %d", workerID, len(blockIDs), blockIDs[0], blockIDs[len(blockIDs)-1])

	// Fetch the blocks using fetchBlockRange
	blocks, err := fetchBlockRange(ctx, blockIDs, config.SidecarURL)
	if err != nil {
		log.Printf("Worker %d error fetching blocks %v: %v", workerID, blockIDs, err)
		return
	}

	// Save the blocks to the database
	if err := saveToDatabase(db, blocks, config); err != nil {
		log.Printf("Worker %d error saving blocks to database: %v", workerID, err)
		return
	}

	log.Printf("Worker %d successfully processed %d blocks", workerID, len(blocks))
}

// processSingleBlock fetches and processes a single block using callSidecar
func processSingleBlock(ctx context.Context, blockID int, config Config, db *sql.DB, workerID int) {
	log.Printf("Worker %d processing single block %d", workerID, blockID)

	// Fetch the block data using callSidecar
	block, err := callSidecar(ctx, blockID, config.SidecarURL)
	if err != nil {
		log.Printf("Worker %d error fetching block %d: %v", workerID, blockID, err)
		return
	}

	// Save the block to the database
	if err := saveToDatabase(db, []BlockData{block}, config); err != nil {
		log.Printf("Worker %d error saving block %d to database: %v", workerID, blockID, err)
		return
	}

	log.Printf("Worker %d successfully processed block %d", workerID, blockID)
}
