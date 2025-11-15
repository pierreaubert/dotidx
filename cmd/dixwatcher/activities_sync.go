package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// SystemHealthResponse represents the JSON-RPC response from system_health
type SystemHealthResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		IsSyncing       bool `json:"isSyncing"`
		Peers           int  `json:"peers"`
		ShouldHavePeers bool `json:"shouldHavePeers"`
	} `json:"result"`
}

// CheckNodeSyncActivity checks if a blockchain node has completed syncing
// Returns true when node is synced (isSyncing=false), false when still syncing
func (a *Activities) CheckNodeSyncActivity(ctx context.Context, rpcEndpoint string, port int) (bool, error) {
	start := time.Now()

	// Build URL
	url := rpcEndpoint
	if url == "" {
		url = fmt.Sprintf("http://localhost:%d", port)
	}

	log.Printf("[Activity] Checking node sync status: %s", url)

	// Prepare JSON-RPC request
	reqBody := map[string]interface{}{
		"id":      1,
		"jsonrpc": "2.0",
		"method":  "system_health",
		"params":  []interface{}{},
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("CheckNodeSync", "error")
			a.metrics.RecordActivityError("CheckNodeSync", "marshal_error")
		}
		return false, fmt.Errorf("failed to marshal JSON-RPC request: %w", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqJSON))
	if err != nil {
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("CheckNodeSync", "error")
			a.metrics.RecordActivityError("CheckNodeSync", "request_error")
		}
		return false, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("CheckNodeSync", "error")
			a.metrics.RecordActivityError("CheckNodeSync", "http_error")
		}
		return false, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("CheckNodeSync", "error")
			a.metrics.RecordActivityError("CheckNodeSync", "http_status_error")
		}
		return false, fmt.Errorf("unexpected HTTP status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var healthResp SystemHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("CheckNodeSync", "error")
			a.metrics.RecordActivityError("CheckNodeSync", "parse_error")
		}
		return false, fmt.Errorf("failed to decode JSON response: %w", err)
	}

	isSynced := !healthResp.Result.IsSyncing
	log.Printf("[Activity] Node %s sync status: isSyncing=%v, peers=%d (synced=%v)",
		url, healthResp.Result.IsSyncing, healthResp.Result.Peers, isSynced)

	// Record metrics
	if a.metrics != nil {
		a.metrics.RecordActivityExecution("CheckNodeSync", "success")
		a.metrics.RecordActivityDuration("CheckNodeSync", time.Since(start))

		// Determine node/chain from URL (simple heuristic)
		nodeName := url
		chainName := "unknown"
		a.metrics.RecordNodeSyncStatus(nodeName, chainName, isSynced, healthResp.Result.Peers)
	}

	return isSynced, nil
}
