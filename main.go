package main

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

	// ----------------------------------------------------------------------
	// Database
	// ----------------------------------------------------------------------
	var db *sql.DB
	if strings.Contains(config.DatabaseURL, "postgres") {
		// Ensure sslmode=disable is in the PostgreSQL URI if not already present
		if !strings.Contains(config.DatabaseURL, "sslmode=") {
			if strings.Contains(config.DatabaseURL, "?") {
				config.DatabaseURL += "&sslmode=disable"
			} else {
				config.DatabaseURL += "?sslmode=disable"
			}
		}

		// Create database connection
		var err error
		db, err = sql.Open("postgres", config.DatabaseURL)
		if err != nil {
			log.Fatalf("Error opening database: %v", err)
		}
		defer db.Close()
	} else {
		log.Fatalf("unsupported database: %s", config.DatabaseURL)
	}

	// Create database instance
	database := NewSQLDatabase(db)

	// Create tables
	if err := database.CreateTable(config); err != nil {
		log.Fatalf("Error creating tables: %v", err)
	}

	// Test the connection
	if err := database.Ping(); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}

	log.Printf("Successfully connected to database %s", config.DatabaseURL)

	// ----------------------------------------------------------------------
	// ChainReader
	// ----------------------------------------------------------------------
	reader := NewSidecar(config.ChainReaderURL)
	// Test the sidecar service
	if err := reader.Ping(); err != nil {
		log.Fatalf("Sidecar service test failed: %v", err)
	}
	log.Println("Sidecar service is working properly")

	// ----------------------------------------------------------------------
	// Set up context with cancellation for graceful shutdown
	// ----------------------------------------------------------------------
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	setupSignalHandler(cancel)

	headBlockID, err := reader.GetChainHeadID()
	if err != nil {
		log.Fatalf("Failed to fetch head block: %v", err)
	}
	log.Printf("Current head block is %d", headBlockID)

	// print some stats
	go NewStats(ctx, database, reader).Print()

	// If in live mode, fetch the head block and update the range
	if config.Live {
		log.Println("Running in live mode")
		// Convert head block ID to integer for range
		// Set the range from 1 to the current head block
		config.StartRange = 1
		config.EndRange = headBlockID

		// Start workers to process existing blocks
		startWorkers(ctx, config, database, reader, headBlockID)

		// Start monitoring for new blocks
		go func() {
			if err := monitorNewBlocks(ctx, config, database, reader, headBlockID); err != nil {
				log.Fatalf("Error monitoring blocks: %v", err)
			}
		}()
	} else {
		// Start workers and wait for completion in normal mode
		startWorkers(ctx, config, database, reader, headBlockID)
	}

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
