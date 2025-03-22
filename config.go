package dotidx

import (
	"flag"
	"time"
)

type Config struct {
	StartRange     int
	EndRange       int
	ChainReaderURL string
	DatabaseURL    string
	BatchSize      int
	MaxWorkers     int
	FlushTimeout   time.Duration
	Relaychain     string
	Chain          string
	Live           bool
}

func ParseFlags() Config {
	chainReaderURL := flag.String("chainreader", "", "Chain reader URL: sidecar or go")
	databaseURL := flag.String("database", "", "Database URL")

	startRange := flag.Int("start", 1, "Start of the integer range")
	endRange := flag.Int("end", 10, "End of the integer range")
	batchSize := flag.Int("batch", 10, "Number of items to collect before writing to database")
	maxWorkers := flag.Int("workers", 5, "Maximum number of concurrent workers")
	flushTimeout := flag.Duration("flush", 30*time.Second, "Maximum time to wait before flushing data to database")

	relaychain := flag.String("relaychain", "Polkadot", "Relaychain name")
	chain := flag.String("chain", "", "Chain name")

	live := flag.Bool("live", false, "Live mode: continuously fetch new blocks as they are produced")

	flag.Parse()

	return Config{
		StartRange:     *startRange,
		EndRange:       *endRange,
		ChainReaderURL: *chainReaderURL,
		DatabaseURL:    *databaseURL,
		BatchSize:      *batchSize,
		MaxWorkers:     *maxWorkers,
		FlushTimeout:   *flushTimeout,
		Relaychain:     *relaychain,
		Chain:          *chain,
		Live:           *live,
	}
}

