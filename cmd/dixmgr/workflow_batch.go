package main

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// BatchWorkflow orchestrates batch block indexing for a specific chain
// This workflow replaces the dixbatch binary functionality using Temporal
func BatchWorkflow(ctx workflow.Context, config BatchWorkflowConfig) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("BatchWorkflow started",
		"relayChain", config.RelayChain,
		"chain", config.Chain,
		"startRange", config.StartRange,
		"endRange", config.EndRange)

	// Configure activity options with retries
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute, // Long timeout for batch operations
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	// Step 1: Get current chain head if EndRange is -1
	endRange := config.EndRange
	if endRange == -1 {
		var headBlock int
		err := workflow.ExecuteActivity(ctx, "GetChainHeadActivity",
			config.SidecarURL).Get(ctx, &headBlock)
		if err != nil {
			logger.Error("Failed to get chain head", "error", err)
			return fmt.Errorf("failed to get chain head: %w", err)
		}
		endRange = headBlock
		logger.Info("Using chain head as end range", "headBlock", headBlock)
	}

	// Step 2: Process blocks in chunks (100k at a time to avoid huge ranges)
	stepRange := 100000
	totalBlocks := endRange - config.StartRange + 1
	logger.Info("Starting batch processing",
		"totalBlocks", totalBlocks,
		"stepRange", stepRange)

	currentStart := config.StartRange
	processedBlocks := 0

	for currentStart <= endRange {
		currentEnd := currentStart + stepRange - 1
		if currentEnd > endRange {
			currentEnd = endRange
		}

		logger.Info("Processing range",
			"start", currentStart,
			"end", currentEnd,
			"blocks", currentEnd-currentStart+1)

		// Step 3: Check which blocks already exist in this range
		var existingBlocks map[int]bool
		err := workflow.ExecuteActivity(ctx, "CheckExistingBlocksActivity",
			config.RelayChain, config.Chain, currentStart, currentEnd).Get(ctx, &existingBlocks)
		if err != nil {
			logger.Error("Failed to check existing blocks", "error", err)
			return fmt.Errorf("failed to check existing blocks: %w", err)
		}

		// Step 4: Identify missing blocks and group into batches
		missingBlocks := []int{}
		for blockID := currentStart; blockID <= currentEnd; blockID++ {
			if !existingBlocks[blockID] {
				missingBlocks = append(missingBlocks, blockID)
			}
		}

		logger.Info("Found missing blocks",
			"count", len(missingBlocks),
			"total", currentEnd-currentStart+1)

		if len(missingBlocks) == 0 {
			logger.Info("No missing blocks in this range, skipping")
			currentStart = currentEnd + 1
			continue
		}

		// Step 5: Process missing blocks in parallel batches
		// Group consecutive blocks into continuous batches
		batches := groupIntoContinuousBatches(missingBlocks, config.BatchSize)

		logger.Info("Processing batches",
			"batchCount", len(batches),
			"maxWorkers", config.MaxWorkers)

		// Process batches with concurrency control
		err = processBatchesConcurrently(ctx, config, batches, logger)
		if err != nil {
			logger.Error("Failed to process batches", "error", err)
			return fmt.Errorf("failed to process batches: %w", err)
		}

		processedBlocks += len(missingBlocks)
		logger.Info("Range processing complete",
			"processedInRange", len(missingBlocks),
			"totalProcessed", processedBlocks)

		// Move to next range
		currentStart = currentEnd + 1

		// Check if we need to use Continue-As-New to avoid history size limits
		// Temporal workflows should use Continue-As-New after processing significant work
		if processedBlocks > 500000 && currentStart <= endRange {
			logger.Info("Using Continue-As-New to reset workflow history",
				"processedSoFar", processedBlocks,
				"remaining", endRange-currentStart+1)

			// Create new config with updated start range
			newConfig := config
			newConfig.StartRange = currentStart

			return workflow.NewContinueAsNewError(ctx, BatchWorkflow, newConfig)
		}
	}

	logger.Info("BatchWorkflow completed successfully",
		"totalProcessed", processedBlocks)

	return nil
}

// groupIntoContinuousBatches groups block IDs into continuous ranges
func groupIntoContinuousBatches(blockIDs []int, batchSize int) [][]int {
	if len(blockIDs) == 0 {
		return [][]int{}
	}

	batches := [][]int{}
	currentBatch := []int{blockIDs[0]}

	for i := 1; i < len(blockIDs); i++ {
		// If continuous and batch not full, add to current batch
		if blockIDs[i] == blockIDs[i-1]+1 && len(currentBatch) < batchSize {
			currentBatch = append(currentBatch, blockIDs[i])
		} else {
			// Start new batch
			batches = append(batches, currentBatch)
			currentBatch = []int{blockIDs[i]}
		}
	}

	// Add last batch
	if len(currentBatch) > 0 {
		batches = append(batches, currentBatch)
	}

	return batches
}

// processBatchesConcurrently processes batches with controlled concurrency
func processBatchesConcurrently(ctx workflow.Context, config BatchWorkflowConfig,
	batches [][]int, logger log.Logger) error {

	// Use Temporal's parallel execution
	// Split work between batch and single block processing
	futures := []workflow.Future{}
	activeFutures := 0

	for _, batch := range batches {
		// Wait if we've reached max workers
		for activeFutures >= config.MaxWorkers {
			// Wait for any future to complete
			workflow.Await(ctx, func() bool {
				// Check if any futures have completed
				for _, f := range futures {
					if f != nil && f.IsReady() {
						return true
					}
				}
				return false
			})
			// At least one future has completed
			activeFutures--
		}

		// Process batch based on size
		var future workflow.Future
		if len(batch) > 1 {
			// Use batch processing for continuous ranges
			future = workflow.ExecuteActivity(ctx, "ProcessBlockBatchActivity",
				config.RelayChain, config.Chain, batch, config.SidecarURL)
		} else {
			// Use single block processing for isolated blocks
			future = workflow.ExecuteActivity(ctx, "ProcessSingleBlockActivity",
				config.RelayChain, config.Chain, batch[0], config.SidecarURL)
		}

		futures = append(futures, future)
		activeFutures++
	}

	// Wait for all remaining futures
	for _, future := range futures {
		if future != nil {
			err := future.Get(ctx, nil)
			if err != nil {
				logger.Error("Batch processing failed", "error", err)
				return err
			}
		}
	}

	return nil
}
