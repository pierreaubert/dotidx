package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"

	dix "github.com/pierreaubert/dotidx"
)

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

	database := dix.NewSQLDatabase(*config)

	if err := database.Ping(); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}

	log.Printf("Successfully connected to database %s", dix.DBUrlSecure(*config))

	ticker := time.NewTicker(1 * time.Hour)
	startCron(context.Background(), ticker, database)
}

func startCron(ctx context.Context, ticker *time.Ticker, db dix.Database) {
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
					info.Chain,
				); err != nil {
					log.Printf("Cannot update stats for %s:%s",
						info.Relaychain,
						info.Chain,
					)
					continue
				}
				log.Printf("Updated stats for %s:%s", info.Relaychain, info.Chain)
			}
		}
	}
}
