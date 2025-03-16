package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Frontend handles the REST API for dotidx
type Frontend struct {
	db             *sql.DB
	config         Config
	listenAddr     string
	metricsHandler *Metrics
	// Cache for expensive queries
	cacheMutex           sync.RWMutex
	monthlyStatsCache    []MonthlyStats
	monthlyStatsCacheExp time.Time
}

// NewFrontend creates a new Frontend instance
func NewFrontend(db *sql.DB, config Config, listenAddr string) *Frontend {
	return &Frontend{
		db:             db,
		config:         config,
		listenAddr:     listenAddr,
		metricsHandler: NewMetrics("Frontend"),
		// Initialize with expired cache
		monthlyStatsCacheExp: time.Now().Add(-1 * time.Hour),
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
		log.Printf("Frontend REST API listening on %s", f.listenAddr)
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

// BlocksResponse is the response for the /address2blocks endpoint
type BlocksResponse struct {
	Blocks []BlockData `json:"blocks"`
}

// CompletionRateResponse is the response for the /stats/completatiorate endpoint
type CompletionRateResponse struct {
	PercentCompletion int `json:"percent_completion"`
	HeadID            int `json:"head_id"`
}

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

	// Convert Block to BlockData for API response
	blockData := make([]BlockData, len(blocks))
	for i, block := range blocks {
		blockData[i] = BlockData{
			ID:             block.ID,
			Timestamp:      block.Timestamp,
			Hash:           block.Hash,
			ParentHash:     block.ParentHash,
			StateRoot:      block.StateRoot,
			ExtrinsicsRoot: block.ExtrinsicsRoot,
			AuthorID:       block.AuthorID,
			Finalized:      block.Finalized,
			OnInitialize:   json.RawMessage(block.OnInitialize),
			OnFinalize:     json.RawMessage(block.OnFinalize),
			Logs:           json.RawMessage(block.Logs),
			Extrinsics:     json.RawMessage(block.Extrinsics),
		}
	}

	// Prepare the response
	response := BlocksResponse{
		Blocks: blockData,
	}

	// Set content type and encode response as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

// handleCompletionRate handles the /stats/completatiorate endpoint
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
	stats, err := f.getCachedMonthlyStats()
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

// getCachedMonthlyStats returns monthly stats with caching (15 min expiration)
func (f *Frontend) getCachedMonthlyStats() ([]MonthlyStats, error) {
	// Check if we have a valid cache
	f.cacheMutex.RLock()
	cachedData := f.monthlyStatsCache
	cacheExpiration := f.monthlyStatsCacheExp
	f.cacheMutex.RUnlock()

	// If cache is still valid, return it
	if time.Now().Before(cacheExpiration) && len(cachedData) > 0 {
		log.Printf("Using cached monthly stats (expires at %s)", cacheExpiration.Format(time.RFC3339))
		return cachedData, nil
	}

	// Cache expired or empty, query the database
	log.Printf("Monthly stats cache expired, querying database")
	stats, err := f.getMonthlyStats()
	if err != nil {
		return nil, err
	}

	// Update cache
	f.cacheMutex.Lock()
	f.monthlyStatsCache = stats
	f.monthlyStatsCacheExp = time.Now().Add(15 * time.Minute)
	f.cacheMutex.Unlock()

	log.Printf("Updated monthly stats cache (expires at %s)", f.monthlyStatsCacheExp.Format(time.RFC3339))
	return stats, nil
}

// getMonthlyStats queries the database to get statistics per month
func (f *Frontend) getMonthlyStats() ([]MonthlyStats, error) {
	// SQL query to get block statistics per month
	query := `
		SELECT
			date_trunc('month',created_at)::date as date,
			count(*),
			min(block_id),
			max(block_id)
		FROM chain.blocks_polkadot_polkadot
		GROUP BY date
		ORDER BY date DESC;
	`

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

// getCompletionRate queries the database to get the completion rate
func (f *Frontend) getCompletionRate() (int, int, error) {
	// SQL query to calculate the completion rate
	query := `
		SELECT
			count(distinct block_id) * 100 / max(block_id) as percentcompletion,
			max(block_id) as head_id
		FROM chain.blocks_polkadot_polkadot;
	`

	// log.Printf("%s", query)

	// Execute the query
	var percentCompletion int
	var headID int
	err := f.db.QueryRow(query).Scan(&percentCompletion, &headID)
	if err != nil {
		return 0, 0, fmt.Errorf("database query failed: %w", err)
	}

	return percentCompletion, headID, nil
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

		// Filter logs - only keep logs containing the address
		block.Logs = filterArrayByAddress(block.Logs, address)

		// Filter on_initialize - only keep events containing the address
		block.OnInitialize = filterObjectWithEvents(block.OnInitialize, address)

		// Filter on_finalize - only keep events containing the address
		block.OnFinalize = filterObjectWithEvents(block.OnFinalize, address)

		// Use recursive filtering for extrinsics due to their complex structure
		block.Extrinsics = filterExtrinsicsRecursive(block.Extrinsics, address)

		blocks = append(blocks, block)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating blocks: %w", err)
	}

	return blocks, nil
}

// filterArrayByAddress filters a JSON array to only include elements containing the specified address
func filterArrayByAddress(data []byte, address string) []byte {
	if len(data) == 0 || !strings.Contains(string(data), address) {
		return []byte("[]")
	}

	// Try to unmarshal as array
	var array []json.RawMessage
	if err := json.Unmarshal(data, &array); err != nil {
		return data // Return original if not a valid array
	}

	// Filter array to only include items containing the address
	filtered := []json.RawMessage{}
	for _, item := range array {
		if strings.Contains(string(item), address) {
			filtered = append(filtered, item)
		}
	}

	result, err := json.Marshal(filtered)
	if err != nil {
		return data // Return original if cannot marshal
	}

	return result
}

// filterObjectWithEvents filters a JSON object, keeping only events containing the specified address
func filterObjectWithEvents(data []byte, address string) []byte {
	if len(data) == 0 || !strings.Contains(string(data), address) {
		return []byte("{}")
	}

	// Parse the JSON object
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return data // Return original if not a valid object
	}

	// Check for events array
	eventsData, ok := obj["events"]
	if !ok {
		return data // Return original if no events field
	}

	// Filter the events array
	obj["events"] = filterArrayByAddress(eventsData, address)

	// Marshal back to JSON
	result, err := json.Marshal(obj)
	if err != nil {
		return data // Return original if cannot marshal
	}

	return result
}

// filterExtrinsicsRecursive recursively filters out extrinsics that don't contain the given address
func filterExtrinsicsRecursive(extrinsicsJson []byte, address string) []byte {
	empty := []byte("[]")
	// Skip empty arrays or if address is not found at all
	if len(extrinsicsJson) == 0 || !strings.Contains(string(extrinsicsJson), address) {
		return empty
	}

	// Parse the JSON into a generic interface
	var extrinsics interface{}
	err := json.Unmarshal(extrinsicsJson, &extrinsics)
	if err != nil {
		log.Printf("Error unmarshaling extrinsics: %v", err)
		return empty
	}

	// Define a recursive function to filter values
	var filterValue func(val interface{}) interface{}
	filterValue = func(val interface{}) interface{} {
		switch val := val.(type) {
		case []interface{}: // Handle array
			result := make([]interface{}, 0)

			// Process each item in the array
			for _, item := range val {
				// Convert item to JSON string to check for direct address match
				jsonData, err := json.Marshal(item)
				if err == nil && strings.Contains(string(jsonData), address) {
					// If item directly contains the address, add it
					result = append(result, item)
					continue
				}

				// Recursively filter the item
				filtered := filterValue(item)
				if filtered != nil {
					// Check if the filtered item contains the address
					jsonData, err := json.Marshal(filtered)
					if err == nil && strings.Contains(string(jsonData), address) {
						result = append(result, filtered)
					}
				}
			}

			// If nothing matched, return nil
			if len(result) == 0 {
				return nil
			}
			return result

		case map[string]interface{}: // Handle object/map
			result := make(map[string]interface{})
			hasAddress := false

			// First, check if any value directly contains the address as a string
			for _, itemValue := range val {
				if str, ok := itemValue.(string); ok {
					if strings.Contains(str, address) {
						hasAddress = true
						// Copy all fields from the original map
						for k, v := range val {
							result[k] = v
						}
						return result
					}
				}
			}

			// Otherwise, recursively filter nested structures (arrays/maps)
			for _, itemValue := range val {
				switch itemValue.(type) {
				case map[string]interface{}, []interface{}:
					// Recursively filter this complex value
					filtered := filterValue(itemValue)

					// If filtered result is not nil, include it and mark this map as containing the address
					if filtered != nil {
						jsonData, err := json.Marshal(filtered)
						if err == nil && strings.Contains(string(jsonData), address) {
							result[""] = filtered
							hasAddress = true
						}
					}
				}
			}

			// If we found the address somewhere in the nested structures, copy all fields from the original map
			if hasAddress {
				// Copy all fields that haven't been added yet
				for k, v := range val {
					if _, exists := result[k]; !exists {
						result[k] = v
					}
				}
				return result
			}

			// If no values contain the address, return nil
			return nil

		default: // Handle primitive values (string, number, boolean, null)
			if str, ok := val.(string); ok {
				// For strings, check direct containment
				if strings.Contains(str, address) {
					return str
				}
			}
			return nil
		}
	}

	// Apply the filter function to the root object
	filtered := filterValue(extrinsics)
	if filtered == nil {
		// If nothing matched, return empty array
		log.Println("Filtered JSON array: null")
		return empty
	}

	// Convert back to JSON
	result, err := json.Marshal(filtered)
	if err != nil {
		log.Printf("Error marshaling filtered extrinsics: %v", err)
		return extrinsicsJson // Return original if cannot marshal
	}

	log.Printf("Filtered extrinsics: %s", result)
	return result
}

// isValidAddress validates a Polkadot address format
func isValidAddress(address string) bool {
	// Polkadot addresses are 47 or 48 characters long and start with a number or letter
	if len(address) < 45 || len(address) > 50 {
		return false
	}

	// Check for common prefixes of Polkadot addresses
	validPrefixes := []string{"1", "5F", "5G", "5D", "5E", "5H"}
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(address, prefix) {
			return true
		}
	}

	return false
}
