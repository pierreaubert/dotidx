package dotidx

import (
	"context"
	"log"
)

// ProcessSingleBlock fetches and processes a single block using fetchBlock
func ProcessSingleBlock(
	ctx context.Context,
	blockID int,
	relayChain, chain string,
	db Database,
	reader ChainReader,
) {
	block, err := reader.FetchBlock(ctx, blockID)
	if err != nil {
		log.Printf("Error fetching block %d: %v", blockID, err)
		return
	}

	// Save block to database
	err = db.Save([]BlockData{block}, relayChain, chain)
	if err != nil {
		log.Printf("Error saving block %d: %v", blockID, err)
		return
	}
}

// ProcessBlockBatch fetches and processes a batch of blocks using fetchBlockRange
func ProcessBlockBatch(
	ctx context.Context,
	blockIDs []int,
	relayChain, chain string,
	db Database,
	reader ChainReader,
) {
	if len(blockIDs) == 0 {
		return
	}

	// Create the array of block IDs from the range
	ids := make([]int, 0, blockIDs[len(blockIDs)-1]-blockIDs[0]+1)
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
	err = db.Save(blockRange, relayChain, chain)
	if err != nil {
		log.Printf("Error saving blocks %d-%d: %v", blockIDs[0], blockIDs[len(blockIDs)-1], err)
		return
	}
}
