package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
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

	config := dotidx.ParseFlags()

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
	frontendAddr := ":8080"
	if len(os.Getenv("FRONTEND_ADDR")) > 0 {
		frontendAddr = os.Getenv("FRONTEND_ADDR")
	}

	// Initialize the frontend server
	frontend := NewFrontend(db, config, frontendAddr)

	// Start the frontend server in a goroutine
	log.Printf("Starting REST API frontend on %s", frontendAddr)
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
}

// NewFrontend creates a new Frontend instance
func NewFrontend(db *sql.DB, config dotidx.Config, listenAddr string) *Frontend {
	return &Frontend{
		db:             db,
		config:         config,
		listenAddr:     listenAddr,
		metricsHandler: dotidx.NewMetrics("Frontend"),
	}
}

// Start initializes and starts the HTTP server
func (f *Frontend) Start(cancelCtx <-chan struct{}) error {
	// Set up the HTTP server
	mux := http.NewServeMux()

	// Serve static files
	// Use absolute path instead of relative path to avoid working directory issues
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	staticDir := filepath.Join(filepath.Dir(execPath), "static")
	log.Printf("Serving static files from: %s", staticDir)
	fs := http.FileServer(http.Dir(staticDir))
	mux.Handle("/", http.StripPrefix("/", fs))

	// Register API routes
	mux.HandleFunc("/address2blocks", f.handleAddressToBlocks)
	mux.HandleFunc("/balances", f.handleBalances)
	mux.HandleFunc("/staking", f.handleStaking)
	mux.HandleFunc("/stats/completion_rate", f.handleCompletionRate)
	mux.HandleFunc("/stats/per_month", f.handleStatsPerMonth)

	// Create HTTP server
	server := &http.Server{
		Addr:    f.listenAddr,
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Wait for cancel context
	<-cancelCtx

	// Shut down the server gracefully
	log.Println("Shutting down frontend server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	return nil
}
