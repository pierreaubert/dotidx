package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pierreaubert/dotidx/dix"
)

// GetChainHeadActivity fetches the current chain head block number
func (a *Activities) GetChainHeadActivity(ctx context.Context, sidecarURL string) (int, error) {
	start := time.Now()
	log.Printf("[Activity] Getting chain head from: %s", sidecarURL)

	// Create a temporary sidecar client
	// Note: In production, we should cache these clients
	sidecar := dix.NewSidecar("", "", sidecarURL)

	headBlock, err := sidecar.GetChainHeadID()
	if err != nil {
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("GetChainHead", "error")
		}
		return 0, fmt.Errorf("failed to get chain head: %w", err)
	}

	if a.metrics != nil {
		a.metrics.RecordActivityExecution("GetChainHead", "success")
		a.metrics.RecordActivityDuration("GetChainHead", time.Since(start))
	}

	log.Printf("[Activity] Chain head: %d", headBlock)
	return headBlock, nil
}

// CheckExistingBlocksActivity checks which blocks already exist in the database
func (a *Activities) CheckExistingBlocksActivity(ctx context.Context,
	relayChain, chain string, startRange, endRange int) (map[int]bool, error) {

	start := time.Now()
	log.Printf("[Activity] Checking existing blocks for %s/%s from %d to %d",
		relayChain, chain, startRange, endRange)

	// Get database instance from config
	// Note: We need to add a database field to Activities
	if a.database == nil {
		return nil, fmt.Errorf("database not configured in activities")
	}

	existingBlocks, err := a.database.GetExistingBlocks(relayChain, chain, startRange, endRange)
	if err != nil {
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("CheckExistingBlocks", "error")
		}
		return nil, fmt.Errorf("failed to check existing blocks: %w", err)
	}

	existingCount := 0
	for _, exists := range existingBlocks {
		if exists {
			existingCount++
		}
	}

	if a.metrics != nil {
		a.metrics.RecordActivityExecution("CheckExistingBlocks", "success")
		a.metrics.RecordActivityDuration("CheckExistingBlocks", time.Since(start))
	}

	log.Printf("[Activity] Found %d existing blocks out of %d total",
		existingCount, endRange-startRange+1)

	return existingBlocks, nil
}

// ProcessSingleBlockActivity fetches and stores a single block
func (a *Activities) ProcessSingleBlockActivity(ctx context.Context,
	relayChain, chain string, blockID int, sidecarURL string) error {

	start := time.Now()
	log.Printf("[Activity] Processing single block %d for %s/%s", blockID, relayChain, chain)

	// Create sidecar client
	sidecar := dix.NewSidecar(relayChain, chain, sidecarURL)

	// Fetch block
	block, err := sidecar.FetchBlock(ctx, blockID)
	if err != nil {
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("ProcessSingleBlock", "error")
		}
		return fmt.Errorf("failed to fetch block %d: %w", blockID, err)
	}

	// Store block in database
	if a.database == nil {
		return fmt.Errorf("database not configured in activities")
	}

	err = a.database.Save([]dix.BlockData{block}, relayChain, chain)
	if err != nil {
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("ProcessSingleBlock", "error")
		}
		return fmt.Errorf("failed to save block %d: %w", blockID, err)
	}

	if a.metrics != nil {
		a.metrics.RecordActivityExecution("ProcessSingleBlock", "success")
		a.metrics.RecordActivityDuration("ProcessSingleBlock", time.Since(start))
	}

	log.Printf("[Activity] Successfully processed block %d", blockID)
	return nil
}

// ProcessBlockBatchActivity fetches and stores a batch of blocks
func (a *Activities) ProcessBlockBatchActivity(ctx context.Context,
	relayChain, chain string, blockIDs []int, sidecarURL string) error {

	start := time.Now()
	log.Printf("[Activity] Processing block batch for %s/%s: %d blocks (range: %d-%d)",
		relayChain, chain, len(blockIDs), blockIDs[0], blockIDs[len(blockIDs)-1])

	if len(blockIDs) == 0 {
		return fmt.Errorf("empty block batch")
	}

	// Create sidecar client
	sidecar := dix.NewSidecar(relayChain, chain, sidecarURL)

	// Fetch batch of blocks
	blocks, err := sidecar.FetchBlockRange(ctx, blockIDs)
	if err != nil {
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("ProcessBlockBatch", "error")
		}
		return fmt.Errorf("failed to fetch block batch: %w", err)
	}

	if len(blocks) != len(blockIDs) {
		log.Printf("[Activity] Warning: requested %d blocks but got %d", len(blockIDs), len(blocks))
	}

	// Store blocks in database
	if a.database == nil {
		return fmt.Errorf("database not configured in activities")
	}

	err = a.database.Save(blocks, relayChain, chain)
	if err != nil {
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("ProcessBlockBatch", "error")
		}
		return fmt.Errorf("failed to save block batch: %w", err)
	}

	if a.metrics != nil {
		a.metrics.RecordActivityExecution("ProcessBlockBatch", "success")
		a.metrics.RecordActivityDuration("ProcessBlockBatch", time.Since(start))
	}

	blocksPerSec := float64(len(blocks)) / time.Since(start).Seconds()
	log.Printf("[Activity] Successfully processed %d blocks (%.2f blocks/sec)",
		len(blocks), blocksPerSec)

	return nil
}
