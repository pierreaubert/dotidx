package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"

	"github.com/pierreaubert/dotidx/dix"
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
	database.CreateTableMonthlyQueryResults()
	addRegisteredQueries()

	if err := database.Ping(); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}

	log.Printf("Successfully connected to database %s", dix.DBUrlSecure(*config))

	cronMonthly := time.NewTicker(24 * time.Hour)
	go func() {
		fillRegisteredQueries(context.Background(), cronMonthly, database)
	}()

	cronTicker := time.NewTicker(1 * time.Hour)
	startCron(context.Background(), cronTicker, database)
}

func addRegisteredQueries() (err error) {

	err = dix.RegisterQuery(
		"total_blocks_in_month",
		`
SELECT COUNT(*) as total_blocks
FROM
  chain.blocks_{{.Relaychain}}_{{.Chain}}
WHERE
  EXTRACT(YEAR FROM created_at) = {{.Year}}
AND
  EXTRACT(MONTH FROM created_at) = {{.Month}};
`,
		"Counts total blocks in a given month and year.",
	)

	if err != nil {
		log.Printf("Error registering query 'total_blocks_in_month': %v", err)
	}

	err = dix.RegisterQuery(
		"total_addresses_in_month",
		`
WITH Boundaries (minBlock, maxBlock) AS (
    SELECT
        MIN(block_id) AS minBlock,
        MAX(block_id) AS maxBlock
    FROM
        chain.blocks_{{.Relaychain}}_{{.Chain}}
    WHERE
        EXTRACT(YEAR FROM created_at) = {{.Year}}
    AND
        EXTRACT(MONTH FROM created_at) = {{.Month}}
)
SELECT
  count(distinct address) AS total_addresses
FROM
  chain.address2blocks_{{.Relaychain}}_{{.Chain}}
WHERE
  block_id <= (SELECT maxBlock FROM Boundaries)
AND
  block_id >= (SELECT minBlock FROM Boundaries)
;
`,
		"Counts unique addresses active in a given month and year.",
	)

	if err != nil {
		log.Printf("Error registering query 'total_addresses_in_month': %v", err)
	}

	return
}

func computeRegisteredQuery(db dix.Database, relayChain, chain string) (err error) {
	firstYear := 2019
	currentYear, currentMonth, _ := time.Now().Date()
	queries, err := dix.GetListOfRegisteredQueries()
	if err != nil {
		return err
	}
	for query := range queries {
		for year := firstYear; year <= currentYear; year++ {
			for month := 1; month <= 12; month++ {
				if year == currentYear && month >= int(currentMonth) {
					continue
				}
				ts, err := db.ReadTimeNamedQuery(context.Background(), relayChain, chain, query.Name, year, month)
				if err == nil {
					if time.Time.IsZero(ts) {
						if err := db.ExecuteAndStoreNamedQuery(context.Background(), relayChain, chain, query.Name, year, month); err != nil {
							log.Printf("Error executing and storing query '%s' for %s/%s - %d/%d: %v",
								query.Name, relayChain, chain, year, month, err)
						}
						log.Printf("Computed %s for %s/%s - %d/%d", query.Name, relayChain, chain, year, month)
					} else {
						log.Printf("Skipped %s for %s/%s - %d/%d", query.Name, relayChain, chain, year, month)
					}
				} else {
					log.Printf("Error %s for %s/%s - %d/%d", query.Name, relayChain, chain, year, month)
				}
			}
		}
	}
	return
}

func computeRegisteredQueries(db dix.Database) {
	infos, err := db.GetDatabaseInfo()
	if err != nil {
		log.Printf("%v", err)
		return
	}
	for i := range infos {
		info := infos[i]
		if err = computeRegisteredQuery(db, info.Relaychain, info.Chain); err != nil {
			log.Printf("Cannot update registered queries for %s:%s",
				info.Relaychain,
				info.Chain,
			)
			continue
		}
		log.Printf("Updated stats for %s:%s", info.Relaychain, info.Chain)
	}
}

func fillRegisteredQueries(ctx context.Context, ticker *time.Ticker, db dix.Database) {
	computeRegisteredQueries(db)
	for {
		select {
		case <-ctx.Done():
			break
		case <-ticker.C:
			computeRegisteredQueries(db)
		}
	}
}

func startCron(ctx context.Context, ticker *time.Ticker, db dix.Database) {
	for {
		select {
		case <-ctx.Done():
			break
		case <-ticker.C:
			infos, err := db.GetDatabaseInfo()
			if err != nil {
				log.Printf("%v", err)
				return
			}
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
