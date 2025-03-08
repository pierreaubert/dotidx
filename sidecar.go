package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Global metrics for sidecar API calls
var sidecarMetrics = NewSidecarMetrics()

// fetchData fetches data for a single block from the sidecar API
func fetchData(ctx context.Context, id int, sidecarURL string) (BlockData, error) {
	return fetchBlock(ctx, id, sidecarURL)
}

// fetchHeadBlock fetches the current head block from the sidecar API
func fetchHeadBlock(sidecarURL string) (int, error) {
	start := time.Now()
	defer func(start time.Time) {
		// log.Printf("Sidecar API call for head block took %v", time.Since(start))
		go func(start time.Time, err error) {
			sidecarMetrics.RecordLatency(start, err)
		}(start, nil)
	}(start)

	// Construct the URL for the head block
	url := fmt.Sprintf("%s/blocks/head", sidecarURL)

	// Make the request
	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("error fetching head block: %w", err)
	}
	defer resp.Body.Close()

	// Check the status code
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("sidecar API returned status code %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading response body for block range: %w", err)
	}

	// Parse the response
	var block BlockData
	if err := json.Unmarshal(body, &block); err != nil {
		return 0, fmt.Errorf("error parsing head block response: %w", err)
	}
	blockID, err := strconv.Atoi(block.ID)
	if err != nil {
		return 0, fmt.Errorf("error parsing head blockID: %w", err)
	}
	return blockID, nil
}

// fetchBlockRange fetches blocks with the specified IDs from the sidecar API
func fetchBlockRange(ctx context.Context, blockIDs []int, sidecarURL string) ([]BlockData, error) {
	start := time.Now()
	defer func(start time.Time) {
		// log.Printf("Sidecar API call for %d blocks took %v", len(blockIDs), time.Since(start))
		go func(start time.Time, err error) {
			sidecarMetrics.RecordLatency(start, err)
		}(start, nil)
	}(start)

	// If no block IDs are provided, return an empty slice
	if len(blockIDs) == 0 {
		return []BlockData{}, nil
	}

	// For now, we'll convert the array to a range query if the blocks are sequential
	// This is more efficient for the API but can be modified later if needed
	isSequential := true
	for i := 1; i < len(blockIDs); i++ {
		if blockIDs[i] != blockIDs[i-1]+1 {
			isSequential = false
			break
		}
	}

	var blocks []BlockData

	if isSequential && len(blockIDs) > 1 {
		// Use range query for sequential blocks
		startID := blockIDs[0]
		endID := blockIDs[len(blockIDs)-1]

		// Construct the URL for the block range
		url := fmt.Sprintf("%s/blocks?range=%d-%d", sidecarURL, startID, endID)

		// Make the request
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating request: %w", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("error fetching block range: %w", err)
		}
		defer resp.Body.Close()

		// Check the status code
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("sidecar API returned status code %d", resp.StatusCode)
		}

		// Read the response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading response body for block range: %w", err)
		}

		// Parse the response
		if err := json.Unmarshal(body, &blocks); err != nil {
			return nil, fmt.Errorf("error parsing block range response: %w", err)
		}
	} else {
		// Fetch blocks individually for non-sequential IDs
		blocks = make([]BlockData, 0, len(blockIDs))
		for _, id := range blockIDs {
			block, err := fetchBlock(ctx, id, sidecarURL)
			if err != nil {
				return nil, fmt.Errorf("error fetching block %d: %w", id, err)
			}
			blocks = append(blocks, block)
		}
	}

	return blocks, nil
}

// fetchBlock makes a call to the sidecar API to fetch a single block
func fetchBlock(ctx context.Context, id int, sidecarURL string) (BlockData, error) {
	start := time.Now()
	defer func(start time.Time) {
		// log.Printf("Sidecar API call for block %d took %v", id, time.Since(start))
		go func(start time.Time, err error) {
			sidecarMetrics.RecordLatency(start, err)
		}(start, nil)
	}(start)

	// Construct the URL for the block
	url := fmt.Sprintf("%s/blocks/%d", sidecarURL, id)

	// Make the request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return BlockData{}, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return BlockData{}, fmt.Errorf("error fetching block %d: %w", id, err)
	}
	defer resp.Body.Close()

	// Check the status code
	if resp.StatusCode != http.StatusOK {
		return BlockData{}, fmt.Errorf("sidecar API returned status code %d for block %d", resp.StatusCode, id)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return BlockData{}, fmt.Errorf("error reading response body for block %d: %w", id, err)
	}

	// Parse the response
	var block BlockData
	if err := json.Unmarshal(body, &block); err != nil {
		return BlockData{}, fmt.Errorf("error parsing response for block %d: %w", id, err)
	}

	return block, nil
}

// testSidecarService tests if the sidecar service is available
func pingSidecarService(sidecarURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", sidecarURL, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error connecting to sidecar service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sidecar service returned status code %d", resp.StatusCode)
	}

	return nil
}
