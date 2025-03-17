package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Frontend handles the REST API for dotidx
type Frontend struct {
	db             *sql.DB
	config         Config
	listenAddr     string
	metricsHandler *Metrics
}

// NewFrontend creates a new Frontend instance
func NewFrontend(db *sql.DB, config Config, listenAddr string) *Frontend {
	return &Frontend{
		db:             db,
		config:         config,
		listenAddr:     listenAddr,
		metricsHandler: NewMetrics("Frontend"),
	}
}

// Start initializes and starts the HTTP server
func (f *Frontend) Start(cancelCtx <-chan struct{}) error {
	// Set up the HTTP server
	mux := http.NewServeMux()

	// Register API routes
	mux.HandleFunc("/address2blocks", f.handleAddressToBlocks)
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

// ----------------------------------------------------------------------
// stats/completion_rate
// ----------------------------------------------------------------------
// CompletionRateResponse is the response for the /stats/completatiorate endpoint
type CompletionRateResponse struct {
	RelayChain        string
	Chain             string
	PercentCompletion float64 `json:"percent_completion"`
	HeadID            int     `json:"head_id"`
}

// getCompletionRate queries the database to get the completion rate
func (f *Frontend) getCompletionRate() (float64, int, error) {

	query := fmt.Sprintf(`SELECT sum(total*100)/max(max_block_id), max(max_block_id) FROM %s;`,
		getStatsPerMonthTableName(f.config))

	log.Printf("%s", query)

	// Execute the query
	var percentCompletion float64
	var headID int
	err := f.db.QueryRow(query).Scan(&percentCompletion, &headID)
	if err != nil {
		return float64(0.0), 0, fmt.Errorf("database query failed: %w", err)
	}

	return percentCompletion, headID, nil
}

func (f *Frontend) handleCompletionRate(w http.ResponseWriter, r *http.Request) {
       // Start timing the request
       startTime := time.Now()
       defer func() {
               f.metricsHandler.RecordLatency(startTime, http.StatusOK, nil)
       }()

       // Only accept GET requests
       if r.Method != "GET" {
               http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
               return
       }

       // Query the database to get the completion rate
       percentCompletion, headID, err := f.getCompletionRate()
       if err != nil {
               log.Printf("Error getting completion rate: %v", err)
               http.Error(w, "Error retrieving completion rate", http.StatusInternalServerError)
               return
       }

       // Prepare response
       response := CompletionRateResponse{
               RelayChain: f.config.Relaychain,
               Chain: f.config.Chain,
               PercentCompletion: percentCompletion,
               HeadID:            headID,
       }

       // Set content type and encode response as JSON
       w.Header().Set("Content-Type", "application/json")
       if err := json.NewEncoder(w).Encode(response); err != nil {
               log.Printf("Error encoding response: %v", err)
               http.Error(w, "Error encoding response", http.StatusInternalServerError)
               return
       }
}



// ----------------------------------------------------------------------
// stats/per_month
// ----------------------------------------------------------------------

// MaxBlockNumberResponse is the response for the /stats/maxblockrate endpoint
type MaxBlockNumberResponse struct {
	MaxBlock int `json:"max_block"`
}

// MonthlyStatsResponse is the response for the /stats/per_month endpoint
type MonthlyStatsResponse struct {
	Data []MonthlyStats `json:"data"`
}

// MonthlyStats represents statistics for a single month
type MonthlyStats struct {
	Date     string `json:"date"`
	Count    int    `json:"count"`
	MinBlock int    `json:"min_block"`
	MaxBlock int    `json:"max_block"`
}

// handleStatsPerMonth handles the /stats/per_month endpoint
func (f *Frontend) handleStatsPerMonth(w http.ResponseWriter, r *http.Request) {
	// Start timing the request
	startTime := time.Now()
	defer func() {
		f.metricsHandler.RecordLatency(startTime, http.StatusOK, nil)
	}()

	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Query the database to get the monthly stats (with caching)
	stats, err := f.getMonthlyStats()
	if err != nil {
		log.Printf("Error getting monthly stats: %v", err)
		http.Error(w, "Error retrieving monthly statistics", http.StatusInternalServerError)
		return
	}

	// Prepare response
	response := MonthlyStatsResponse{
		Data: stats,
	}

	// Set content type and encode response as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

// getMonthlyStats queries the database to get statistics per month
func (f *Frontend) getMonthlyStats() ([]MonthlyStats, error) {
	// SQL query to get block statistics per month
	query := fmt.Sprintf(`
		SELECT *
		FROM %s;
	`, getStatsPerMonthTableName(f.config))

	// log.Printf("%s", query)

	// Execute the query
	rows, err := f.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}
	defer rows.Close()

	// Process results
	var stats []MonthlyStats
	for rows.Next() {
		var stat MonthlyStats
		var date time.Time

		// Scan the row into variables
		err := rows.Scan(&date, &stat.Count, &stat.MinBlock, &stat.MaxBlock)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		// Format the date as YYYY-MM
		stat.Date = date.Format("2006-01")

		stats = append(stats, stat)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return stats, nil
}



// ----------------------------------------------------------------------
// address2blocks
// ----------------------------------------------------------------------

// BlocksResponse is the response for the /address2blocks endpoint
type BlocksResponse struct {
	Blocks []BlockData `json:"blocks"`
}

// handleAddressToBlocks handles the /address2blocks endpoint
func (f *Frontend) handleAddressToBlocks(w http.ResponseWriter, r *http.Request) {
	// Start timing the request
	startTime := time.Now()
	defer func() {
		f.metricsHandler.RecordLatency(startTime, http.StatusOK, nil)
	}()

	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get address from query parameter
	address := r.URL.Query().Get("address")
	if address == "" {
		http.Error(w, "Missing address parameter", http.StatusBadRequest)
		return
	}

	// Validate address format (simple validation)
	if !isValidAddress(address) {
		http.Error(w, "Invalid address format", http.StatusBadRequest)
		return
	}

	// Get blocks for the address
	blocks, err := f.getBlocksByAddress(address)
	if err != nil {
		log.Printf("Error getting blocks for address %s: %v", address, err)
		http.Error(w, "Error retrieving blocks", http.StatusInternalServerError)
		return
	}

	// Set content type and encode response as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(blocks); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

// getBlocksByAddress queries the database to find blocks associated with the given address
func (f *Frontend) getBlocksByAddress(address string) ([]BlockData, error) {
	// Validate the address string before proceeding
	if !isValidAddress(address) {
		return nil, fmt.Errorf("invalid address format")
	}

	query := fmt.Sprintf(
		`SELECT
			b.block_id,
			b.created_at,
			b.hash,
			b.parent_hash,
			b.state_root,
			b.extrinsics_root,
			b.author_id,
			b.finalized,
			b.on_initialize,
			b.on_finalize,
			b.logs,
			b.extrinsics
		FROM %s b
		JOIN %s a ON b.block_id = a.block_id
		WHERE a.address = '%s'
		ORDER BY b.block_id DESC
		LIMIT 10;`,
		getBlocksTableName(f.config),
		getAddressTableName(f.config),
		address,
	)

	rows, err := f.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}
	defer rows.Close()

	var blocks []BlockData

	for rows.Next() {
		var block BlockData
		err = rows.Scan(
			&block.ID,
			&block.Timestamp,
			&block.Hash,
			&block.ParentHash,
			&block.StateRoot,
			&block.ExtrinsicsRoot,
			&block.AuthorID,
			&block.Finalized,
			&block.OnInitialize,
			&block.OnFinalize,
			&block.Logs,
			&block.Extrinsics,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning block: %w", err)
		}

		blocks = append(blocks, block)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating blocks: %w", err)
	}

	return blocks, nil
}
