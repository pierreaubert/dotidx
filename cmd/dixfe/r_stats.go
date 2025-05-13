package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/pierreaubert/dotidx/dix"
)

type CompletionRateResponse struct {
	RelayChain        string
	Chain             string
	PercentCompletion float64 `json:"percent_completion"`
	HeadID            int     `json:"head_id"`
}

func (f *Frontend) getCompletionRate(relaychain, chain string) (float64, int, error) {

	headUrl := fmt.Sprintf("%s/blocks/head/header", f.sidecars[relaychain][chain])

	req, err := http.NewRequest("GET", headUrl, nil)
	if err != nil {
		return 0.0, 0, fmt.Errorf("Failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0.0, 0, fmt.Errorf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	// Check the status code
	if resp.StatusCode != http.StatusOK {
		return 0.0, 0, fmt.Errorf("sidecar API returned status code %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0.0, 0, fmt.Errorf("error reading response body for block range: %w", err)
	}

	var headHeader map[string]any
	if err = json.Unmarshal(body, &headHeader); err != nil {
		return 0.0, 0, fmt.Errorf("Failed to unmarshall response: %v", err)
	}

	numberValue, ok := headHeader["number"]
	if !ok {
		return 0.0, 0, fmt.Errorf("JSON response header missing 'number' field")
	}

	numberInt, ok := numberValue.(string)
	if !ok {
		return 0.0, 0, fmt.Errorf("JSON field 'number' is not (string), got %T", numberValue)
	}

	headID := 0
	headID, err = strconv.Atoi(numberInt)
	if err != nil {
		return 0.0, 0, fmt.Errorf("Failed to parse number: %v", err)
	}

	if headID == 0 {
		return 0.0, 0, fmt.Errorf("head ID is 0")
	}

	query := fmt.Sprintf(
		`
SELECT
  sum((results -> 0 -> 'total_blocks')::int)
FROM
  chain.dotidx_monthly_query_results
WHERE
  relay_chain = '%s'
AND
  chain = '%s'
AND
  query_name = 'total_blocks_in_month'
`,
		relaychain, chain)

	log.Printf("%s", query)

	var count int
	err = f.db.QueryRow(query).Scan(&count)
	if err != nil {
		return float64(0.0), 0, fmt.Errorf("database query failed: %w", err)
	}

	percentCompletion := 100.0 * float64(count) / float64(headID)
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

	infos, err := f.database.GetDatabaseInfo()

	if err != nil {
		log.Printf("No chain infos found")
		http.Error(w, "No chain infos found", http.StatusInternalServerError)
		return
	}

	responses := make([]CompletionRateResponse, len(infos))

	for i := range infos {
		if percentCompletion, headID, err := f.getCompletionRate(infos[i].Relaychain, infos[i].Chain); err == nil {
			response := CompletionRateResponse{
				RelayChain:        infos[i].Relaychain,
				Chain:             infos[i].Chain,
				PercentCompletion: percentCompletion,
				HeadID:            headID,
			}
			responses[i] = response
		}
	}

	// Set content type and encode response as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(responses); err != nil {
		log.Printf("Error encoding responses: %v", err)
		http.Error(w, "Error encoding responses", http.StatusInternalServerError)
		return
	}
}

type MonthlyStats struct {
	Relaychain string
	Chain      string
	Date       string `json:"date"`
	Count      int    `json:"count"`
	MinBlock   int    `json:"min_block"`
	MaxBlock   int    `json:"max_block"`
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

	infos, err := f.database.GetDatabaseInfo()

	if err != nil {
		log.Printf("No chain infos found")
		http.Error(w, "No chain infos found", http.StatusInternalServerError)
		return
	}

	responses := make([]MonthlyStats, 0)

	for i := range infos {

		stats, err := f.getMonthlyStats(infos[i].Relaychain, infos[i].Chain)
		if err != nil {
			log.Printf("Error getting monthly stats: %v", err)
			http.Error(w, "Error retrieving monthly statistics", http.StatusInternalServerError)
			return
		}

		for j := range stats {
			response := MonthlyStats{
				Relaychain: infos[i].Relaychain,
				Chain:      infos[i].Chain,
				Date:       stats[j].Date,
				Count:      stats[j].Count,
				MinBlock:   stats[j].MinBlock,
				MaxBlock:   stats[j].MaxBlock,
			}

			responses = append(responses, response)
		}
	}

	// Set content type and encode response as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(responses); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

// getMonthlyStats queries the database to get statistics per month
func (f *Frontend) getMonthlyStats(relaychain, chain string) ([]MonthlyStats, error) {
	// SQL query to get block statistics per month
	query := fmt.Sprintf(`
		SELECT *
		FROM %s;
	`, dix.GetStatsPerMonthTableName(relaychain, chain))

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
