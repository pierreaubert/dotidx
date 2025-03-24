package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"

	"github.com/pierreaubert/dotidx"
)

func validateConfig(config dotidx.Config) error {

	// In live mode, we don't need to validate the range as it will be determined dynamically
	if !config.Live {
		if config.StartRange > config.EndRange {
			return fmt.Errorf("start range must be less than or equal to end range")
		}
	}

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
	database := dotidx.NewSQLDatabase(config)

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

	if err := database.Ping(); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}

	log.Printf("Successfully connected to database %s", config.DatabaseURL)

	// ----------------------------------------------------------------------
	// Monitoring
	// ----------------------------------------------------------------------
	log.Println("Starting monitoring for new blocks...")
	if err := monitorNewBlocks(ctx, config, database, reader, headBlockID); err != nil {
		log.Fatalf("Error monitoring blocks: %v", err)
	}

}

// MonitorNewBlocks continuously monitors for new blocks and adds them to the database
func monitorNewBlocks(
	ctx context.Context,
	config dotidx.Config,
	db dotidx.Database,
	reader dotidx.ChainReader,
	lastProcessedBlock int) error {
	log.Printf("Starting to monitor new blocks from block %d", lastProcessedBlock)

	nextBlockID := lastProcessedBlock + 1
	currentHeadID := lastProcessedBlock

	ticker := time.NewTicker(1 * time.Second)
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
				log.Printf("Current head is at block %d, next to process is block %d", headID, nextBlockID)
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
				err = db.Save([]dotidx.BlockData{block}, config)
				if err != nil {
					log.Printf("Error saving block %d: %v", nextBlockID, err)
					break
				}

				log.Printf("Processed block %d", nextBlockID)
				nextBlockID++
			}
		}
	}
}
