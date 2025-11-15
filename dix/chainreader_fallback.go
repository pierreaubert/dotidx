package dix

import (
	"context"
	"fmt"
	"log"
)

// FallbackChainReader implements ChainReader with a fallback mechanism
// It tries the primary reader first, and falls back to the secondary reader on failure
type FallbackChainReader struct {
	relay     string
	chain     string
	primary   ChainReader
	secondary ChainReader
	metrics   *Metrics
}

// NewFallbackChainReader creates a new FallbackChainReader
// wsUrl: WebSocket URL for the primary SubstrateRPC reader
// httpUrl: HTTP URL for the fallback Sidecar reader
func NewFallbackChainReader(relay, chain, wsUrl, httpUrl string) *FallbackChainReader {
	primary := NewSubstrateRPCReader(relay, chain, wsUrl)
	secondary := NewSidecar(relay, chain, httpUrl)

	return &FallbackChainReader{
		relay:     relay,
		chain:     chain,
		primary:   primary,
		secondary: secondary,
		metrics:   NewMetrics("FallbackChainReader"),
	}
}

// GetChainHeadID implements ChainReader interface with fallback
func (f *FallbackChainReader) GetChainHeadID() (int, error) {
	// Try primary reader first
	headID, err := f.primary.GetChainHeadID()
	if err == nil {
		return headID, nil
	}

	log.Printf("Primary reader failed for %s:%s GetChainHeadID: %v, falling back to secondary", f.relay, f.chain, err)

	// Fall back to secondary reader
	headID, err = f.secondary.GetChainHeadID()
	if err != nil {
		return -1, fmt.Errorf("both primary and secondary readers failed: %w", err)
	}

	return headID, nil
}

// FetchBlock implements ChainReader interface with fallback
func (f *FallbackChainReader) FetchBlock(ctx context.Context, id int) (BlockData, error) {
	// Try primary reader first
	block, err := f.primary.FetchBlock(ctx, id)
	if err == nil {
		return block, nil
	}

	log.Printf("Primary reader failed for %s:%s FetchBlock(%d): %v, falling back to secondary", f.relay, f.chain, id, err)

	// Fall back to secondary reader
	block, err = f.secondary.FetchBlock(ctx, id)
	if err != nil {
		return BlockData{}, fmt.Errorf("both primary and secondary readers failed for block %d: %w", id, err)
	}

	return block, nil
}

// FetchBlockRange implements ChainReader interface with fallback
func (f *FallbackChainReader) FetchBlockRange(ctx context.Context, blockIDs []int) ([]BlockData, error) {
	// Try primary reader first
	blocks, err := f.primary.FetchBlockRange(ctx, blockIDs)
	if err == nil {
		return blocks, nil
	}

	log.Printf("Primary reader failed for %s:%s FetchBlockRange: %v, falling back to secondary", f.relay, f.chain, err)

	// Fall back to secondary reader
	blocks, err = f.secondary.FetchBlockRange(ctx, blockIDs)
	if err != nil {
		return nil, fmt.Errorf("both primary and secondary readers failed for block range: %w", err)
	}

	return blocks, nil
}

// Ping implements ChainReader interface with fallback
func (f *FallbackChainReader) Ping() error {
	// Try primary reader first
	err := f.primary.Ping()
	if err == nil {
		log.Printf("Primary reader (SubstrateRPC) is available for %s:%s", f.relay, f.chain)
		return nil
	}

	log.Printf("Primary reader ping failed for %s:%s: %v, trying secondary", f.relay, f.chain, err)

	// Try secondary reader
	err = f.secondary.Ping()
	if err != nil {
		return fmt.Errorf("both primary and secondary readers failed ping: %w", err)
	}

	log.Printf("Secondary reader (Sidecar) is available for %s:%s", f.relay, f.chain)
	return nil
}

// GetStats implements ChainReader interface
// Returns stats from the primary reader (SubstrateRPC)
func (f *FallbackChainReader) GetStats() *MetricsStats {
	// Return primary stats since it's the one we primarily use
	return f.primary.GetStats()
}
