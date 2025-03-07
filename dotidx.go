package dotidx

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"strings"

	_ "github.com/lib/pq"
)

func main() {
	// Parse command line arguments
	config := parseFlags()

	// Set up logging
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Validate configuration
	if err := validateConfig(config); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Ensure sslmode=disable is in the PostgreSQL URI if not already present
	if !strings.Contains(config.PostgresURI, "sslmode=") {
		if strings.Contains(config.PostgresURI, "?") {
			config.PostgresURI += "&sslmode=disable"
		} else {
			config.PostgresURI += "?sslmode=disable"
		}
	}

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", config.PostgresURI)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	// Test the connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}

	log.Println("Successfully connected to PostgreSQL")

	// Create table if not exists
	if err := createTable(db, config); err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
	log.Println("Ensured table exists")

	// Test the sidecar service
	if err := testSidecarService(config.SidecarURL); err != nil {
		log.Fatalf("Sidecar service test failed: %v", err)
	}
	log.Println("Sidecar service is working properly")

	// Set up context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	setupSignalHandler(cancel)

	// If in live mode, fetch the head block and update the range
	if config.Live {
		log.Println("Running in live mode")
		headBlock, err := fetchHeadBlock(config.SidecarURL)
		if err != nil {
			log.Fatalf("Failed to fetch head block: %v", err)
		}
		log.Printf("Current head block is %d", headBlock)

		// Set the range from 1 to the current head block
		config.StartRange = 1
		config.EndRange = headBlock

		// Start workers to process existing blocks
		startWorkers(ctx, config, db)

		// Start monitoring for new blocks
		monitorNewBlocks(ctx, config, db, headBlock)
	} else {
		// Start workers and wait for completion in normal mode
		startWorkers(ctx, config, db)
	}

	sidecarMetrics.PrintStats()

	if !config.Live {
		log.Println("All tasks completed")
	}
}

func setupSignalHandler(cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Println("Received interrupt signal, shutting down...")
		cancel()
	}()
}
