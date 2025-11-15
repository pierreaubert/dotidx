package dix

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"time"

	substrate "github.com/itering/substrate-api-rpc"
	"github.com/itering/substrate-api-rpc/metadata"
	"github.com/itering/substrate-api-rpc/model"
	rpc "github.com/itering/substrate-api-rpc/rpc"
	"github.com/itering/substrate-api-rpc/storageKey"
	rpcutil "github.com/itering/substrate-api-rpc/util"
	"github.com/itering/substrate-api-rpc/websocket"
)

// SubstrateRPCReader implements ChainReader using the Go substrate-rpc-api library
// This provides a native Go alternative to the HTTP-based Sidecar service
type SubstrateRPCReader struct {
	relay       string
	chain       string
	wsUrl       string
	metadatas   map[int]*metadata.Instant
	runtimes    map[string]RuntimeVersion
	metrics     *Metrics
	initialized bool
}

// RuntimeVersion represents the runtime version information
type RuntimeVersion struct {
	SpecName           string  `json:"specName"`
	SpecVersion        int     `json:"specVersion"`
	ImplName           string  `json:"implName"`
	ImplVersion        int     `json:"implVersion"`
	AuthoringVersion   int     `json:"authoringVersion"`
	TransactionVersion int     `json:"transactionVersion"`
	StateVersion       int     `json:"stateVersion"`
	APIs               [][]any `json:"apis"`
}

// EncodedDigest represents the digest in a block header
type EncodedDigest struct {
	Logs []string `json:"logs"`
}

// EncodedHeader represents the block header
type EncodedHeader struct {
	Number         string        `json:"number"`
	ParentHash     string        `json:"parentHash"`
	StateRoot      string        `json:"stateRoot"`
	ExtrinsicsRoot string        `json:"extrinsicsRoot"`
	Digest         EncodedDigest `json:"digest"`
}

// EncodedBlock represents a block received via RPC/WS
type EncodedBlock struct {
	Block struct {
		Header     EncodedHeader `json:"header"`
		Extrinsics []string      `json:"extrinsics"`
	} `json:"block"`
	Justifications any `json:"justifications"`
}

// NewSubstrateRPCReader creates a new SubstrateRPCReader instance
func NewSubstrateRPCReader(relay, chain, wsUrl string) *SubstrateRPCReader {
	return &SubstrateRPCReader{
		relay:       relay,
		chain:       chain,
		wsUrl:       wsUrl,
		metadatas:   make(map[int]*metadata.Instant),
		runtimes:    make(map[string]RuntimeVersion),
		metrics:     NewMetrics("SubstrateRPC"),
		initialized: false,
	}
}

// initialize connects to the WebSocket and fetches initial runtime and metadata
func (r *SubstrateRPCReader) initialize(blockID int) error {
	if r.initialized {
		return nil
	}

	websocket.SetEndpoint(r.wsUrl)

	blockHash, err := rpc.GetChainGetBlockHash(nil, blockID)
	if err != nil {
		return fmt.Errorf("failed to get block %d hash: %w", blockID, err)
	}

	runtime, err := r.getRuntime(blockID, blockHash)
	if err != nil {
		return err
	}

	r.runtimes["relay-chain"] = runtime

	meta, err := r.getMetadata(runtime.SpecVersion, blockHash)
	if err != nil {
		return err
	}

	r.metadatas[runtime.SpecVersion] = meta
	r.initialized = true

	return nil
}

// getRuntime fetches the runtime version for a specific block
func (r *SubstrateRPCReader) getRuntime(blockID int, blockHash string) (RuntimeVersion, error) {
	var rpcRuntimeResult model.JsonRpcResult
	runtimeRequest := rpc.ChainGetRuntimeVersion(blockID, blockHash)
	err := websocket.SendWsRequest(nil, &rpcRuntimeResult, runtimeRequest)
	if err != nil {
		return RuntimeVersion{}, fmt.Errorf("failed to send runtime version request: %w", err)
	}
	if rpcRuntimeResult.Error != nil {
		return RuntimeVersion{}, fmt.Errorf("RPC error fetching runtime version: %v", rpcRuntimeResult.Error)
	}
	if rpcRuntimeResult.Result == nil {
		return RuntimeVersion{}, fmt.Errorf("received nil result for runtime version")
	}

	var resultBytes []byte
	resultBytes, err = json.Marshal(rpcRuntimeResult.Result)
	if err != nil {
		return RuntimeVersion{}, fmt.Errorf("failed to marshal runtime version result: %w", err)
	}

	var runtimeVersion RuntimeVersion
	err = json.Unmarshal(resultBytes, &runtimeVersion)
	if err != nil {
		return RuntimeVersion{}, fmt.Errorf("failed to unmarshal runtime version: %w", err)
	}

	if runtimeVersion.SpecVersion == 0 {
		return RuntimeVersion{}, fmt.Errorf("received runtime version with specVersion 0")
	}

	return runtimeVersion, nil
}

// getMetadata fetches the metadata for a specific spec version
func (r *SubstrateRPCReader) getMetadata(specVersion int, blockHash string) (*metadata.Instant, error) {
	rawMetadata, err := rpc.GetMetadataByHash(nil, blockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata by hash %s: %w", blockHash, err)
	}
	if rawMetadata == "" {
		return nil, fmt.Errorf("received empty metadata for hash %s", blockHash)
	}

	meta := metadata.RegNewMetadataType(specVersion, rawMetadata)
	if meta == nil {
		return nil, fmt.Errorf("failed to process metadata for spec %d", specVersion)
	}

	return meta, nil
}

// GetChainHeadID implements ChainReader interface
func (r *SubstrateRPCReader) GetChainHeadID() (int, error) {
	start := time.Now()
	defer func(start time.Time) {
		go func(start time.Time, err error) {
			r.metrics.RecordLatency(start, 1, err)
		}(start, nil)
	}(start)

	// Ensure initialized
	if !r.initialized {
		if err := r.initialize(1); err != nil {
			return -1, fmt.Errorf("failed to initialize: %w", err)
		}
	}

	blockHash, err := rpc.GetChainGetBlockHash(nil, -1) // -1 gets the latest block
	if err != nil {
		return -1, fmt.Errorf("failed to get head block hash: %w", err)
	}

	var rpcBlockResult model.JsonRpcResult
	blockRequest := rpc.ChainGetBlock(rand.Intn(10000), blockHash)
	err = websocket.SendWsRequest(nil, &rpcBlockResult, blockRequest)
	if err != nil {
		return -1, fmt.Errorf("failed to get head block: %w", err)
	}

	if rpcBlockResult.Error != nil {
		return -1, fmt.Errorf("RPC error fetching head block: %v", rpcBlockResult.Error)
	}

	if rpcBlockResult.Result == nil {
		return -1, fmt.Errorf("received nil result for head block")
	}

	result, ok := rpcBlockResult.Result.(map[string]interface{})
	if !ok {
		return -1, fmt.Errorf("unexpected result type for head block")
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return -1, fmt.Errorf("failed to marshal head block result: %w", err)
	}

	var encodedBlock EncodedBlock
	err = json.Unmarshal(resultBytes, &encodedBlock)
	if err != nil {
		return -1, fmt.Errorf("failed to unmarshal head block: %w", err)
	}

	blockNum, err := strconv.ParseInt(encodedBlock.Block.Header.Number, 0, 64)
	if err != nil {
		return -1, fmt.Errorf("failed to parse block number: %w", err)
	}

	return int(blockNum), nil
}

// FetchBlock implements ChainReader interface
func (r *SubstrateRPCReader) FetchBlock(ctx context.Context, id int) (BlockData, error) {
	start := time.Now()
	defer func(start time.Time) {
		go func(start time.Time, err error) {
			r.metrics.RecordLatency(start, 1, err)
		}(start, nil)
	}(start)

	// Ensure initialized
	if !r.initialized {
		if err := r.initialize(id); err != nil {
			return BlockData{}, fmt.Errorf("failed to initialize: %w", err)
		}
	}

	// Get block hash
	hash, err := rpc.GetChainGetBlockHash(nil, id)
	if err != nil {
		return BlockData{}, fmt.Errorf("failed to get block %d hash: %w", id, err)
	}

	// Fetch block details
	encodedBlock, err := r.fetchBlockDetails(hash, id)
	if err != nil {
		return BlockData{}, fmt.Errorf("error fetching block details for %d: %w", id, err)
	}

	// Fetch events
	encodedEvents, err := r.fetchEvents(hash, id)
	if err != nil {
		return BlockData{}, fmt.Errorf("error fetching events for block %d: %w", id, err)
	}

	// Get runtime info
	runtimeInfo, ok := r.runtimes["relay-chain"]
	if !ok {
		return BlockData{}, fmt.Errorf("runtime info not found for block %d", id)
	}

	// Get metadata
	meta, ok := r.metadatas[runtimeInfo.SpecVersion]
	if !ok {
		return BlockData{}, fmt.Errorf("metadata for spec version %d not found", runtimeInfo.SpecVersion)
	}

	// Decode extrinsics
	extrinsics, err := r.decodeExtrinsics(id, encodedBlock.Block.Extrinsics, meta, runtimeInfo.SpecVersion)
	if err != nil {
		return BlockData{}, err
	}

	// Decode events
	events, err := r.decodeEvents(id, encodedEvents, meta, runtimeInfo.SpecVersion)
	if err != nil {
		// Events may be empty, don't fail
		events = []map[string]interface{}{}
	}

	// Build block data
	block := r.buildBlockData(id, hash, encodedBlock, extrinsics, events)

	return block, nil
}

// FetchBlockRange implements ChainReader interface
func (r *SubstrateRPCReader) FetchBlockRange(ctx context.Context, blockIDs []int) ([]BlockData, error) {
	if len(blockIDs) == 0 {
		return []BlockData{}, nil
	}

	start := time.Now()
	defer func(start time.Time) {
		go func(start time.Time, err error) {
			r.metrics.RecordLatency(start, len(blockIDs), err)
		}(start, nil)
	}(start)

	blocks := make([]BlockData, 0, len(blockIDs))
	for _, id := range blockIDs {
		select {
		case <-ctx.Done():
			return blocks, ctx.Err()
		default:
			block, err := r.FetchBlock(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("error fetching block %d: %w", id, err)
			}
			blocks = append(blocks, block)
		}
	}

	return blocks, nil
}

// Ping implements ChainReader interface
func (r *SubstrateRPCReader) Ping() error {
	// Try to get chain head to verify connection
	_, err := r.GetChainHeadID()
	return err
}

// GetStats implements ChainReader interface
func (r *SubstrateRPCReader) GetStats() *MetricsStats {
	return r.metrics.GetStats()
}

// fetchBlockDetails fetches the full block details
func (r *SubstrateRPCReader) fetchBlockDetails(blockHash string, blockNum int) (EncodedBlock, error) {
	blockRequest := rpc.ChainGetBlock(rand.Intn(10000), blockHash)
	var rpcBlockResult model.JsonRpcResult
	err := websocket.SendWsRequest(nil, &rpcBlockResult, blockRequest)
	if err != nil {
		return EncodedBlock{}, fmt.Errorf("failed to send block request: %w", err)
	}

	if rpcBlockResult.Error != nil {
		return EncodedBlock{}, fmt.Errorf("RPC error fetching block: %v", rpcBlockResult.Error)
	}

	if rpcBlockResult.Result == nil {
		return EncodedBlock{}, fmt.Errorf("received nil result for block")
	}

	result, ok := rpcBlockResult.Result.(map[string]interface{})
	if !ok {
		return EncodedBlock{}, fmt.Errorf("unexpected result type for block")
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return EncodedBlock{}, fmt.Errorf("failed to marshal block result: %w", err)
	}

	var blockResponse EncodedBlock
	err = json.Unmarshal(resultBytes, &blockResponse)
	if err != nil {
		return EncodedBlock{}, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	return blockResponse, nil
}

// fetchEvents fetches events for a block
func (r *SubstrateRPCReader) fetchEvents(blockHash string, blockNum int) (string, error) {
	var rpcEventResult model.JsonRpcResult

	eventsKeyBytes := storageKey.EncodeStorageKey("System", "Events")
	storageRequest := rpc.StateGetStorage(
		rand.Intn(10000),
		rpcutil.AddHex(eventsKeyBytes.EncodeKey),
		blockHash)

	err := websocket.SendWsRequest(nil, &rpcEventResult, storageRequest)
	if err != nil {
		return "", fmt.Errorf("failed to send event storage request: %w", err)
	}

	if rpcEventResult.Error != nil {
		return "", fmt.Errorf("RPC error fetching events: %v", rpcEventResult.Error)
	}

	if rpcEventResult.Result == nil {
		return "", nil // No events
	}

	resultBytes, err := json.Marshal(rpcEventResult.Result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal event storage result: %w", err)
	}

	var rawEventsHex string
	err = json.Unmarshal(resultBytes, &rawEventsHex)
	if err != nil {
		return "", nil // Empty events
	}

	return rawEventsHex, nil
}

// decodeExtrinsics decodes extrinsics from raw data
func (r *SubstrateRPCReader) decodeExtrinsics(
	blockNum int,
	extrinsics []string,
	meta *metadata.Instant,
	specVersion int,
) ([]map[string]interface{}, error) {
	decodedExtrinsicData, err := substrate.DecodeExtrinsic(extrinsics, meta, specVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to decode extrinsics: %w", err)
	}
	return decodedExtrinsicData, nil
}

// decodeEvents decodes events from raw hex data
func (r *SubstrateRPCReader) decodeEvents(
	blockNum int,
	rawEventsHex string,
	meta *metadata.Instant,
	specVersion int,
) ([]map[string]interface{}, error) {
	if rawEventsHex == "" || rawEventsHex == "0x" {
		return nil, nil
	}

	decodedEvents, err := substrate.DecodeEvent(rawEventsHex, meta, specVersion)
	if err != nil {
		return nil, fmt.Errorf("decoding events failed: %w", err)
	}

	eventsInterface, ok := decodedEvents.([]interface{})
	if !ok {
		return nil, fmt.Errorf("decoded events are not a slice, type: %T", decodedEvents)
	}

	var events []map[string]interface{}
	for i, e := range eventsInterface {
		ems, ok := e.(map[string]interface{})
		if !ok {
			log.Printf("[Event %d-%d] Warning: Event casting failed", blockNum, i)
			continue
		}
		events = append(events, ems)
	}

	return events, nil
}

// buildBlockData constructs a BlockData from decoded information
func (r *SubstrateRPCReader) buildBlockData(
	blockNum int,
	hash string,
	encodedBlock EncodedBlock,
	extrinsics []map[string]interface{},
	events []map[string]interface{},
) BlockData {
	block := BlockData{
		ID:             strconv.Itoa(blockNum),
		Hash:           hash,
		ParentHash:     encodedBlock.Block.Header.ParentHash,
		StateRoot:      encodedBlock.Block.Header.StateRoot,
		ExtrinsicsRoot: encodedBlock.Block.Header.ExtrinsicsRoot,
		Logs:           json.RawMessage("[]"),
		Extrinsics:     json.RawMessage("[]"),
		AuthorID:       "",
		Finalized:      true,
		OnInitialize:   nil,
		OnFinalize:     nil,
	}

	// Map events per module
	eventsSet := make(map[string][]int)
	for e := range events {
		if key, ok := events[e]["call_module"].(string); ok {
			eventsSet[key] = append(eventsSet[key], e)
		}
	}

	// Process extrinsics
	for _, extrinsic := range extrinsics {
		// Extract timestamp
		if callModule, ok := extrinsic["call_module"].(string); ok && callModule == "Timestamp" {
			if params, ok := extrinsic["params"].([]interface{}); ok && len(params) > 0 {
				if data, ok := params[0].(map[string]interface{}); ok {
					if now, ok := data["value"].(float64); ok {
						snow := strconv.Itoa(int(now) / 1000)
						if blockTimestamp, err := ParseTimestamp(snow); err == nil {
							block.Timestamp = blockTimestamp
						}
					}
				}
			}
		}

		// Handle era/mortal
		encodedEra, _ := extrinsic["era"].(string)
		storageMortal := substrate.DecodeMortal(encodedEra)
		blockMortal := make(map[string]interface{})
		if storageMortal == nil {
			blockMortal["immortalArea"] = "0x00"
		} else {
			blockMortal["mortalArea"] = []uint64{storageMortal.Period, storageMortal.Phase}
		}
		delete(extrinsic, "era")
		extrinsic["era"] = blockMortal

		// Merge events
		if callModule, ok := extrinsic["call_module"].(string); ok {
			if eventIndexes, ok := eventsSet[callModule]; ok && len(eventIndexes) > 0 {
				var relevant []map[string]interface{}
				for _, i := range eventIndexes {
					relevant = append(relevant, events[i])
				}
				if eventsJSON, err := json.Marshal(relevant); err == nil {
					extrinsic["events"] = eventsJSON
				}
			}
		}

		// Remove raw fields
		for k := range extrinsic {
			if len(k) > 4 && k[len(k)-4:] == "_raw" {
				delete(extrinsic, k)
			}
		}
	}

	// Marshal extrinsics
	if extrinsicsJSON, err := json.Marshal(extrinsics); err == nil {
		block.Extrinsics = extrinsicsJSON
	}

	// Handle logs (digest)
	if storageLogs, err := substrate.DecodeLogDigest(encodedBlock.Block.Header.Digest.Logs); err == nil {
		if logsJSON, err := json.Marshal(storageLogs); err == nil {
			block.Logs = logsJSON
		}
	}

	return block
}
