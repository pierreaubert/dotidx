package dix

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
	// gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
)

type ChainReader interface {
	GetChainHeadID() (int, error)
	FetchBlockRange(ctx context.Context, blockIDs []int) ([]BlockData, error)
	FetchBlock(ctx context.Context, id int) (BlockData, error)
	Ping() error
	GetStats() *MetricsStats
}

// Sidecar implements the ChainReader interface using Substrate API Sidecar
// Supports both regular blocks and elastic scaling enabled parachains
// Note: Elastic scaling support (v20.9.0+) allows multiple blocks per block height
// The database schema uses (hash, created_at) as primary key to handle this
type Sidecar struct {
	relay   string
	chain   string
	url     string
	metrics *Metrics
}

func NewSidecar(relay, chain, url string) *Sidecar {
	return &Sidecar{
		relay:   relay,
		chain:   chain,
		url:     url,
		metrics: NewMetrics("Sidecar"),
	}
}

// fetchHeadBlock fetches the current head block from the sidecar API
func (s *Sidecar) GetChainHeadID() (int, error) {

	start := time.Now()
	defer func(start time.Time) {
		go func(start time.Time, err error) {
			s.metrics.RecordLatency(start, 1, err)
		}(start, nil)
	}(start)

	// Construct the URL for the head block
	url := fmt.Sprintf("%s/blocks/head", s.url)

	// Make the request
	resp, err := http.Get(url)
	if err != nil {
		return -1, fmt.Errorf("error fetching head block: %w", err)
	}
	defer resp.Body.Close()

	// Check the status code
	if resp.StatusCode != http.StatusOK {
		return -1, fmt.Errorf("sidecar API for (%s, %s) returned status code %d", s.relay, s.chain, resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return -1, fmt.Errorf("error reading response body for block range: %w", err)
	}

	// Parse the response
	var block BlockData
	if err = json.Unmarshal(body, &block); err != nil {
		return -1, fmt.Errorf("error parsing head block response: %w", err)
	}
	blockID, err := strconv.Atoi(block.ID)
	if err != nil {
		return 0, fmt.Errorf("error parsing head blockID: %w", err)
	}
	return blockID, nil
}

// fetchBlockRange fetches blocks with the specified IDs from the sidecar API
func (s *Sidecar) FetchBlockRange(ctx context.Context, blockIDs []int) ([]BlockData, error) {

	// If no block IDs are provided, return an empty slice
	if len(blockIDs) == 0 {
		return []BlockData{}, nil
	}

	start := time.Now()
	defer func(start time.Time) {
		go func(start time.Time, err error) {
			s.metrics.RecordLatency(start, len(blockIDs), err)
		}(start, nil)
	}(start)

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
		url := fmt.Sprintf("%s/blocks?range=%d-%d", s.url, startID, endID)

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
			block, err := s.FetchBlock(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("error fetching block %d: %w", id, err)
			}
			blocks = append(blocks, block)
		}
	}

	return blocks, nil
}

// fetchBlock makes a call to the sidecar API to fetch a single block
// Note: With elastic scaling, multiple blocks may exist at the same height
// This function returns the canonical block. For multi-block queries, use useRcBlock parameter
func (s *Sidecar) FetchBlock(ctx context.Context, id int) (BlockData, error) {
	start := time.Now()
	defer func(start time.Time) {
		go func(start time.Time, err error) {
			s.metrics.RecordLatency(start, 1, err)
		}(start, nil)
	}(start)

	// Construct the URL for the block
	url := fmt.Sprintf("%s/blocks/%d", s.url, id)

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
func (s *Sidecar) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", s.url, nil)
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

func (s *Sidecar) GetStats() *MetricsStats {
	return s.metrics.GetStats()
}

// NewChainReader creates a ChainReader with fallback support
// It uses SubstrateRPC as primary and HTTP Sidecar as fallback
func NewChainReader(relay, chain, wsUrl, httpUrl string) ChainReader {
	return NewFallbackChainReader(relay, chain, wsUrl, httpUrl)
}

// NewChainReaderFromConfig creates a ChainReader from ParaChainConfig
// It automatically constructs the WebSocket and HTTP URLs from config
func NewChainReaderFromConfig(relay, chain string, config ParaChainConfig) ChainReader {
	// Determine the node IP
	nodeIP := config.NodeIP
	if nodeIP == "" {
		nodeIP = config.RelayIP
	}
	if nodeIP == "" {
		nodeIP = "127.0.0.1"
	}

	// Construct WebSocket URL for SubstrateRPC
	wsUrl := fmt.Sprintf("ws://%s:%d", nodeIP, config.PortWS)

	// Construct HTTP URL for Sidecar fallback
	httpUrl := fmt.Sprintf("http://%s:%d", config.ChainreaderIP, config.ChainreaderPort)

	return NewChainReader(relay, chain, wsUrl, httpUrl)
}
