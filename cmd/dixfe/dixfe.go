package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/pierreaubert/dotidx"
)

func validateConfig(config dotidx.Config) error {

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

	config, err := dotidx.ParseFlags()
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
	dotidx.SetupSignalHandler(cancel)

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

	database := dotidx.NewSQLDatabaseWithDB(db)
	if err := database.Ping(); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}
	log.Printf("Successfully connected to database %s", config.DatabaseURL)

	// ----------------------------------------------------------------------
	// REST Frontend
	// ----------------------------------------------------------------------
	frontend := NewFrontend(db, config)

	if err := frontend.Start(ctx.Done()); err != nil {
		log.Printf("Error starting frontend server: %v", err)
	}
}

// Frontend handles the REST API for dotidx
type Frontend struct {
	db             *sql.DB
	config         dotidx.Config
	listenAddr     string
	metricsHandler *dotidx.Metrics
	staticPath     string
}

// NewFrontend creates a new Frontend instance
func NewFrontend(db *sql.DB, config dotidx.Config) *Frontend {
	listenAddr := fmt.Sprintf("%s:%d", config.FrontendIP, config.FrontendPort)
	return &Frontend{
		db:             db,
		config:         config,
		listenAddr:     listenAddr,
		metricsHandler: dotidx.NewMetrics("Frontend"),
		staticPath:     config.FrontendStatic,
	}
}

// Start initializes and starts the HTTP server
func (f *Frontend) Start(cancelCtx <-chan struct{}) error {
	mux := http.NewServeMux()

	log.Printf("Serving static files from: %s", f.staticPath)
	fs := http.FileServer(http.Dir(f.staticPath))
	mux.Handle("/", http.StripPrefix("/", fs))

	mux.HandleFunc("/address2blocks", f.handleAddressToBlocks)
	mux.HandleFunc("/balances", f.handleBalances)
	mux.HandleFunc("/staking", f.handleStaking)
	mux.HandleFunc("/stats/completion_rate", f.handleCompletionRate)
	mux.HandleFunc("/stats/per_month", f.handleStatsPerMonth)

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
