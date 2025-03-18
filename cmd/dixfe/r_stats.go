package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/pierreaubert/dotidx"
)

type CompletionRateResponse struct {
	RelayChain        string
	Chain             string
	PercentCompletion float64 `json:"percent_completion"`
	HeadID            int     `json:"head_id"`
}

func (f *Frontend) getCompletionRate() (float64, int, error) {

	query := fmt.Sprintf(`SELECT sum(total*100)/max(max_block_id), max(max_block_id) FROM %s;`,
		dotidx.GetStatsPerMonthTableName(f.config))

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
		RelayChain:        f.config.Relaychain,
		Chain:             f.config.Chain,
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

type MaxBlockNumberResponse struct {
	MaxBlock int `json:"max_block"`
}

type MonthlyStatsResponse struct {
	Data []MonthlyStats `json:"data"`
}

type MonthlyStats struct {
	Date     string `json:"date"`
	Count    int    `json:"count"`
	MinBlock int    `json:"min_block"`
	MaxBlock int    `json:"max_block"`
}

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
	`, dotidx.GetStatsPerMonthTableName(f.config))

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
