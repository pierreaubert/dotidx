package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
	dix "github.com/pierreaubert/dotidx"
)

func main() {

	configFile := flag.String("conf", "", "toml configuration file")
	flag.Parse()

	config, err := dix.LoadMgrConfig(*configFile)
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dix.SetupSignalHandler(cancel)

	var db *sql.DB
	databaseURL := dix.DBUrl(*config)

	if strings.Contains(databaseURL, "postgres") {
		db, err = sql.Open("postgres", databaseURL)
		if err != nil {
			log.Fatalf("Error opening database: %v", err)
		}
		defer db.Close()
	} else {
		log.Fatalf("unsupported database: %s", dix.DBUrlSecure(*config))
	}

	database := dix.NewSQLDatabaseWithDB(db)
	if err := database.Ping(); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}
	log.Printf("Successfully connected to database %s", dix.DBUrlSecure(*config))

	// ----------------------------------------------------------------------
	// REST Frontend
	// ----------------------------------------------------------------------
	frontend := NewFrontend(database, db, *config)

	if err := frontend.Start(ctx.Done()); err != nil {
		log.Printf("Error starting frontend server: %v", err)
	}
}

// Frontend handles the REST API for dix
type Frontend struct {
	// abstraction
	database *dix.SQLDatabase
	// underlying db
	db *sql.DB
	// general configuration
	config dix.MgrConfig
	// address where FE is exposed
	listenAddr string
	// 1 only for the whole FE
	metricsHandler *dix.Metrics
	// path to the directory with the static files
	// it is for convenience and not having to spin a reverse proxy in dev mode
	staticPath string
	// a list of proxys one per chain
	proxys map[string]map[string]*httputil.ReverseProxy
}

// NewFrontend creates a new Frontend instance
func NewFrontend(database *dix.SQLDatabase, db *sql.DB, config dix.MgrConfig) *Frontend {
	listenAddr := fmt.Sprintf(`%s:%d`, config.DotidxFE.IP, config.DotidxFE.Port)
	proxys := make(map[string]map[string]*httputil.ReverseProxy)
	for relay := range config.Parachains {
		proxys[relay] = make(map[string]*httputil.ReverseProxy)
		for chain := range config.Parachains[relay] {
			ip := config.Parachains[relay][chain].ChainreaderIP
			port := config.Parachains[relay][chain].ChainreaderPort
			remote, _ := url.Parse(fmt.Sprintf("http://%s:%d", ip, port))
			proxy := httputil.NewSingleHostReverseProxy(remote)
			proxys[relay][chain] = proxy
		}
	}
	return &Frontend{
		database:       database,
		db:             db,
		config:         config,
		listenAddr:     listenAddr,
		metricsHandler: dix.NewMetrics("Frontend"),
		staticPath:     config.DotidxFE.StaticPath,
		proxys:         proxys,
	}
}

// Start initializes and starts the HTTP server
func (f *Frontend) Start(cancelCtx <-chan struct{}) error {
	mux := http.NewServeMux()

	log.Printf("Serving at http://%s/index.html", f.listenAddr)

	// serving static files for convenience
	log.Printf("Serving static files from: %s", f.staticPath)
	fs := http.FileServer(http.Dir(f.staticPath))

	mux.Handle("GET /index.html", http.StripPrefix("/", fs))
	mux.Handle("GET /", http.StripPrefix("/", fs))

	// fe functions
	mux.HandleFunc("GET /fe/address2blocks", f.handleAddressToBlocks)
	mux.HandleFunc("GET /fe/balances", f.handleBalances)
	mux.HandleFunc("GET /fe/staking", f.handleStaking)
	mux.HandleFunc("GET /fe/stats/completion_rate", f.handleCompletionRate)
	mux.HandleFunc("GET /fe/stats/per_month", f.handleStatsPerMonth)
	// per chain
	mux.HandleFunc("GET /fe/{relay}/{chain}/blocks/{blockid}", f.handleBlock)
	// proxy to sidecar
	mux.HandleFunc("GET /proxy/{relay}/{chain}/accounts/{address}/balance-info", f.handleProxy)

	server := &http.Server{
		Addr:    f.listenAddr,
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Wait for cancel context
	<-cancelCtx

	log.Println("Shutting down frontend server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	return nil
}
