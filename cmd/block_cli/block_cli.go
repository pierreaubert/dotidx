package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"github.com/pierreaubert/dotidx/dix"
)


func main() {
	wsURL := flag.String("ws", "", "WebSocket endpoint URL (required)")
	startBlockNum := flag.Int("start", 0, "Start block number")
	blockCount := flag.Int("count", 1, "Number of blocks to process")
	printOutput := flag.Bool("print", false, "Print decoded extrinsics and events")

	flag.Parse()

	if *wsURL == "" {
		fmt.Println("WebSocket URL (-ws) is required")
		flag.Usage()
		return
	}

	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)

	// Create SubstrateRPC reader
	reader := dix.NewSubstrateRPCReader("relay-chain", "test-chain", *wsURL)

	// Test connection
	log.Println("Testing connection to WebSocket endpoint...")
	if err := reader.Ping(); err != nil {
		log.Fatalf("Failed to connect to WebSocket endpoint: %v", err)
	}
	log.Println("Successfully connected to WebSocket endpoint")

	// Get chain head
	headBlockID, err := reader.GetChainHeadID()
	if err != nil {
		log.Fatalf("Failed to fetch head block: %v", err)
	}
	log.Printf("Chain head block: %d", headBlockID)

	// Process blocks
	ctx := context.Background()
	failedBlocks := 0
	successBlocks := 0

	for i := 0; i < *blockCount; i++ {
		blockNum := *startBlockNum + i
		log.Printf("Fetching block %d...", blockNum)

		block, err := reader.FetchBlock(ctx, blockNum)
		if err != nil {
			log.Printf("ERROR: Failed to get block %d: %v", blockNum, err)
			failedBlocks++
			continue
		}

		successBlocks++
		log.Printf("Successfully fetched block %d (hash: %s)", blockNum, block.Hash)

		if *printOutput {
			jsBlock, err := json.MarshalIndent(block, "", "  ")
			if err != nil {
				log.Printf("ERROR: Failed to marshal block %d: %v", blockNum, err)
				continue
			}
			fmt.Printf("%s\n", string(jsBlock))
		}
	}

	// Print stats
	stats := reader.GetStats()
	log.Printf("\n=== Summary ===")
	log.Printf("Processed: %d blocks", successBlocks)
	log.Printf("Failed: %d blocks", failedBlocks)

	// Use the most recent bucket (last minute)
	if len(stats.BucketsStats) > 0 {
		recentStats := stats.BucketsStats[3] // Last minute bucket
		log.Printf("Recent stats (1 minute):")
		log.Printf("  Total calls: %d", recentStats.Count)
		log.Printf("  Failures: %d", recentStats.Failures)
		log.Printf("  Average latency: %v", recentStats.Avg)
		log.Printf("  Min latency: %v", recentStats.Min)
		log.Printf("  Max latency: %v", recentStats.Max)
		log.Printf("  Rate: %.2f calls/sec", recentStats.Rate)
	}
}
