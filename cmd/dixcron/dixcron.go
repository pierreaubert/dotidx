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

	if config.DatabaseURL == "" {
		return fmt.Errorf("database url is required")
	}

	return nil
}

func main() {
	// Parse command line arguments
	config, err := dotidx.ParseFlags()
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

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
	database := dotidx.NewSQLDatabase(config)

	// Test the connection
	if err := database.Ping(); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}

	log.Printf("Successfully connected to database %s", config.DatabaseURL)

	ticker := time.NewTicker(1 * time.Hour)
	startCron(context.Background(), ticker, database)
	log.Println("All tasks completed")
}

func startCron(ctx context.Context, ticker *time.Ticker, db dotidx.Database) {
	infos, err := db.GetDatabaseInfo()
	if err != nil {
		log.Printf("%v", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			break
		case <-ticker.C:
			for i := range infos {
				info := infos[i]
				if err = db.UpdateMaterializedTables(
					info.Relaychain,
					info.Chain); err != nil {
					log.Printf("Cannot update stats for %s:%s",
						info.Relaychain,
						info.Chain)

				}
				log.Printf("Updated stats for %s:%s", info.Relaychain, info.Chain)
			}
		}
	}
}
