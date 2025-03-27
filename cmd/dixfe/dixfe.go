package main

import (
	"context"
	"database/sql"
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

func validateConfig(config dix.Config) error {

	if config.ChainReaderURL == "" {
		return fmt.Errorf("chainReader url is required")
	}

	if config.DatabaseURL == "" {
		return fmt.Errorf("database url is required")
	}

	if config.Chain == "" {
		return fmt.Errorf("chain name is required")
	}

	return nil
}

func main() {

	config, err := dix.ParseFlags()
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	if err := validateConfig(config); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dix.SetupSignalHandler(cancel)

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

	database := dix.NewSQLDatabaseWithDB(db)
	if err := database.Ping(); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}
	log.Printf("Successfully connected to database %s", config.DatabaseURL)

	// ----------------------------------------------------------------------
	// REST Frontend
	// ----------------------------------------------------------------------
	frontend := NewFrontend(database, db, config)

	if err := frontend.Start(ctx.Done()); err != nil {
		log.Printf("Error starting frontend server: %v", err)
	}
}

// Frontend handles the REST API for dix
type Frontend struct {
	database       *dix.SQLDatabase
	db             *sql.DB
	config         dix.Config
	listenAddr     string
	metricsHandler *dix.Metrics
	staticPath     string
	proxy          *httputil.ReverseProxy
}

// NewFrontend creates a new Frontend instance
func NewFrontend(database *dix.SQLDatabase, db *sql.DB, config dix.Config) *Frontend {
	listenAddr := fmt.Sprintf("%s:%d", config.FrontendIP, config.FrontendPort)
	remote, _ := url.Parse(config.ChainReaderURL)
	proxy := httputil.NewSingleHostReverseProxy(remote)
	return &Frontend{
		database:       database,
		db:             db,
		config:         config,
		listenAddr:     listenAddr,
		metricsHandler: dix.NewMetrics("Frontend"),
		staticPath:     config.FrontendStatic,
		proxy:          proxy,
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
	mux.HandleFunc("GET /fe/blocks/{blockid}", f.handleBlock)
	mux.HandleFunc("GET /fe/staking", f.handleStaking)
	mux.HandleFunc("GET /fe/stats/completion_rate", f.handleCompletionRate)
	mux.HandleFunc("GET /fe/stats/per_month", f.handleStatsPerMonth)
	// proxy to sidecar
	mux.HandleFunc("GET /proxy/accounts/{address}/balance-info", f.handleProxy)

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
