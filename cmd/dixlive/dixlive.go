package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"

	dix "github.com/pierreaubert/dotidx"
)

type ChainState struct {
	reader  *dix.Sidecar
	current int
	head    int
}

func main() {
	configFile := flag.String("conf", "", "toml configuration file")
	flag.Parse()

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

	log.Printf("Starting continous head blocks ingestion")

	// ----------------------------------------------------------------------
	// Set up context with cancellation for graceful shutdown
	// ----------------------------------------------------------------------
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	dix.SetupSignalHandler(cancel)

	// ----------------------------------------------------------------------
	// ChainReader
	// ----------------------------------------------------------------------
	readers := make(map[string]map[string]*ChainState)

	for relayChain := range config.Parachains {
		readers[relayChain] = make(map[string]*ChainState)
		for chain := range config.Parachains[relayChain] {
			ip := config.Parachains[relayChain][chain].ChainreaderIP
			port := config.Parachains[relayChain][chain].ChainreaderPort
			url := fmt.Sprintf("http://%s:%d", ip, port)
			reader := dix.NewSidecar(relayChain, chain, url)
			if err := reader.Ping(); err != nil {
				log.Printf("Sidecar service test for %s:%s failed: %v", relayChain, chain, err)
				continue
			}
			headBlockID, err := reader.GetChainHeadID()
			if err != nil {
				log.Printf("Failed to fetch head block for %s:%s: %v", relayChain, chain, err)
				continue
			}
			log.Printf("Sidecar is up for %s:%s head is at %d", relayChain, chain, headBlockID)
			readers[relayChain][chain] = &ChainState{
				reader:  reader,
				current: headBlockID,
				head:    headBlockID,
			}
		}
	}

	// ----------------------------------------------------------------------
	// Database
	// ----------------------------------------------------------------------
	database := dix.NewSQLDatabase(*config)
	if err := database.Ping(); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}
	log.Printf("Successfully connected to database %s", dix.DBUrl(*config))

	for relayChain := range readers {
		for chain := range readers[relayChain] {
			reader := readers[relayChain][chain].reader
			head := readers[relayChain][chain].head
			first, err := reader.FetchBlock(ctx, 1)
			if err != nil {
				log.Fatalf("Cannot get block 1: %v", err)
			}
			firstTimestamp, err := dix.ExtractTimestamp(first.Extrinsics)
			if err != nil {
				// some parachain do not have the pallet timestamp
				firstTimestamp = ""
			}
			lastBlock, err := reader.FetchBlock(ctx, head)
			if err != nil {
				log.Fatalf("Cannot get head block %d: %v", head, err)
			}
			lastTimestamp, err := dix.ExtractTimestamp(lastBlock.Extrinsics)
			if err != nil {
				lastTimestamp = time.Now().Format("2006-01-02 15:04:05")
			}
			if err := database.CreateTable(relayChain, chain, firstTimestamp, lastTimestamp); err != nil {
				log.Fatalf("Error creating tables: %v", err)
			}
		}
	}

	// ----------------------------------------------------------------------
	// Monitoring
	// ----------------------------------------------------------------------
	log.Println("Starting monitoring for new blocks...")
	if err := monitorNewBlocks(ctx, *config, database, readers); err != nil {
		log.Fatalf("Error monitoring blocks: %v", err)
	}

}

func processLastBlocks(
	relayChain, chain string,
	ctx context.Context,
	db dix.Database,
	state *ChainState,
) error {
	head, err := state.reader.GetChainHeadID()
	if err != nil {
		log.Printf("Error fetching head block: %v", err)
		return err
	}

	next := state.current

	log.Printf("Processing %12s:%12s:%10d+%d", relayChain, chain, next, head-next)

	for next <= head {
		block, err := state.reader.FetchBlock(ctx, next)
		if err != nil {
			log.Printf("Error fetching block %d: %v", next, err)
			break
		}
		err = db.Save([]dix.BlockData{block}, relayChain, chain)
		if err != nil {
			log.Printf("Error saving block %d: %v", next, err)
			break
		}
		next++
	}
	state.current = head
	return nil
}

// MonitorNewBlocks continuously monitors for new blocks and adds them to the database
func monitorNewBlocks(
	ctx context.Context,
	config dix.MgrConfig,
	db dix.Database,
	readers map[string]map[string]*ChainState,
) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			for relayChain := range readers {
				for chain := range readers[relayChain] {
					go processLastBlocks(
						relayChain,
						chain,
						ctx,
						db,
						readers[relayChain][chain],
					)
				}
			}
		}
	}
}
