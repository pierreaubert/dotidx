package dotidx

import (
	"flag"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
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
	FrontendIP     string
	FrontendPort   int
	FrontendStatic string
}

func checkPortFollowConvention(chainreaderUrl string, expectedPort int) bool {
	url, err := url.Parse(chainreaderUrl)
	if err != nil {
		return false
	}
	_, port, _ := net.SplitHostPort(url.Host)
	iport, err := strconv.Atoi(port)
	if err != nil {
		return false
	}
	return iport == expectedPort
}

func ParseFlags() (Config, error) {

	chainReaderURL := flag.String("chainreader", "", "Chain reader URL: sidecar or go")
	databaseURL := flag.String("database", "", "Database URL")

	startRange := flag.Int("start", 1, "Start of the integer range")
	endRange := flag.Int("end", -1, "End of the integer range. If not set head of the chain block id will be used")

	batchSize := flag.Int("batch", 10, "Number of items to collect before writing to database")
	maxWorkers := flag.Int("workers", 5, "Maximum number of concurrent workers")
	flushTimeout := flag.Duration("flush", 30*time.Second, "Maximum time to wait before flushing data to database")

	relaychain := flag.String("relaychain", "Polkadot", "Relaychain name")
	chain := flag.String("chain", "", "Chain name")

	live := flag.Bool("live", false, "Live mode: continuously fetch new blocks as they are produced")

	frontendIP := flag.String("frontend-ip", "127.0.0.1", "IP address of the frontend")
	frontendPort := flag.Int("frontend-port", 8080, "IP port of the frontend")
	frontendStatic := flag.String("frontend-static", "static", "path to the static html/css/js files of the frontend")
	flag.Parse()

	// add some checks here to minimise risk of writing to the wrong database
	if *chainReaderURL != "" {
		if strings.ToLower(*relaychain) == "polkadot" {
			expectedPort := 0
			switch strings.ToLower(*chain) {
			case "polkadot":
				expectedPort = 10800
			case "assethub":
				expectedPort = 10900
			case "people":
				expectedPort = 11000
			case "collectives":
				expectedPort = 11100
			case "mythical":
				expectedPort = 11200
			case "frequency":
				expectedPort = 11300
			}
			if !checkPortFollowConvention(*chainReaderURL, expectedPort) {
				return Config{}, fmt.Errorf("%s:%s sidecar port should be %d got %s",
					strings.ToLower(*relaychain),
					strings.ToLower(*chain),
					10800, *chainReaderURL)
			}
		}
	}

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
		FrontendIP:     *frontendIP,
		FrontendPort:   *frontendPort,
		FrontendStatic: *frontendStatic,
	}, nil
}
