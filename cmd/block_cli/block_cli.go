package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"maps"
	"math/rand"
	"slices"
	"strconv"
	"time"

	substrate "github.com/itering/substrate-api-rpc"
	"github.com/itering/substrate-api-rpc/metadata"
	"github.com/itering/substrate-api-rpc/model"
	rpc "github.com/itering/substrate-api-rpc/rpc"
	// "github.com/itering/substrate-api-rpc/storage"
	"github.com/itering/substrate-api-rpc/storageKey"
	rpcutil "github.com/itering/substrate-api-rpc/util"
	ss58 "github.com/itering/substrate-api-rpc/util/ss58"
	"github.com/itering/substrate-api-rpc/websocket"

	dix "github.com/pierreaubert/dotidx"
)

// Define structs based on typical JSON RPC responses if not provided by the library
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

type EncodedDigest struct {
	Logs []string `json:"logs"`
}

type EncodedHeader struct {
	Number         string   `json:"number"`
	ParentHash     string `json:"parentHash"`
	StateRoot      string `json:"stateRoot"`
	ExtrinsicsRoot string `json:"extrinsicsRoot"`
	Digest         EncodedDigest    `json:"digest"`
}

// block encoded received by rpc/ws
type EncodedBlock struct {
	Block struct {
		Header     EncodedHeader `json:"header"`
		Extrinsics []string    `json:"extrinsics"`
	} `json:"block"`
	Justifications string         `json:"justifications"` // if signed
}

// for manipulation of event, extrinsic and log data
type BlockLog struct {
	Type string      `json:"type"`
	Index int         `json:"index"`
	Value []string    `json:"value"`
}

type BlockDigest struct {
	Logs []*BlockLog `json:"logs"`
}

type Extrinsic = map[string]interface{}
type Event = map[string]interface{}

// convenience fucntions
func interface2string(i interface{}) (val string) {
	switch i := i.(type) {
	case string:
		val = i
	case []byte:
		val = string(i)
	default:
		b, _ := json.Marshal(i)
		val = string(b)
	}
	return
}

func interface2array(i interface{}) (val []string) {
	switch i := i.(type) {
	case []interface{}:
		for _, e := range i {
			val = append(val, interface2string(e))
		}
	default:
		b := interface2string(i)
		val = append(val, string(b))
	}
	return
}

func getRuntime(blockID int, blockHash string) (runtimeVersion RuntimeVersion, err error) {
	var rpcRuntimeResult model.JsonRpcResult
	runtimeRequest := rpc.ChainGetRuntimeVersion(blockID, blockHash)
	err = websocket.SendWsRequest(nil, &rpcRuntimeResult, runtimeRequest)
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
		return RuntimeVersion{}, fmt.Errorf("failed to marshal runtime version result interface: %w", err)
	}
	err = json.Unmarshal(resultBytes, &runtimeVersion)
	if err != nil {
		return RuntimeVersion{}, fmt.Errorf("failed to unmarshal runtime version result from JSON: %w", err)
	}
	specVersion := runtimeVersion.SpecVersion
	if specVersion == 0 {
		return RuntimeVersion{}, fmt.Errorf("received runtime version with specVersion 0")
	}
	return runtimeVersion, nil
}

func getMetadata(specVersion int, blockHash string) (meta *metadata.Instant, err error) {
	rawMetadata, err := rpc.GetMetadataByHash(nil, blockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata by hash %s: %w", blockHash, err)
	}
	if rawMetadata == "" {
		return nil, fmt.Errorf("received empty metadata for hash %s", blockHash)
	}
	meta = metadata.RegNewMetadataType(specVersion, rawMetadata)
	if meta == nil {
		return nil, fmt.Errorf("failed to process metadata for spec %d, hash %s", specVersion, blockHash)
	}
	return meta, nil
}

func getValidators(blockHash string) (list []string, err error) {
	rawValidators, err := rpc.ReadStorage(nil, "Session", "Validators", blockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get Validators from storage %s: %w", blockHash, err)
	}
	var sliceValidators []string
	err = json.Unmarshal([]byte(rawValidators), &sliceValidators)
	for _, v := range sliceValidators {
		list = append(list, rpcutil.TrimHex(v))
	}
	return list, nil
}

func extractTimestamp(extrinsicName string, extrinsic Extrinsic) (blockTimestamp time.Time, err error) {
	call_module := extrinsic["call_module"]
	if call_module != "Timestamp" {
		return time.Time{}, fmt.Errorf("call module %s not Timestamp", call_module)
	}

	params, ok := extrinsic["params"].([]interface{})
	if !ok || len(params) == 0 {
		return time.Time{}, fmt.Errorf("call module %s has no params", call_module)
	}
	data, ok := params[0].(map[string]interface{})
	if !ok {
		return time.Time{}, fmt.Errorf("call module %s has no params data", call_module)
	}
	now, ok := data["value"].(float64)
	if !ok {
		return time.Time{}, fmt.Errorf("call module %s has no now", call_module)
	}
	snow := strconv.Itoa(int(now) / 1000)
	blockTimestamp, err = dix.ParseTimestamp(snow)
	if err != nil {
		return time.Time{}, err
	}
	return
}

// sidecar clone
type GoSidecar struct {
	wsUrl       string
	metadatas   map[int]*metadata.Instant
	runtimes    map[string]RuntimeVersion
	printTraces bool // emit logs for tracing in/out/success
	printDebug  bool // print extrinsics and events
	printOutput bool // print decoded extrinsics and events
}

// NewGoSidecar creates a new GoSidecar instance
func NewGoSidecar(wsUrl string, printTraces bool, printDebug bool, printOutput bool) *GoSidecar {
	gsc := &GoSidecar{
		wsUrl:       wsUrl,
		metadatas:   make(map[int]*metadata.Instant),
		runtimes:    make(map[string]RuntimeVersion),
		printTraces: printTraces,
		printDebug:  printDebug,
		printOutput: printOutput,
	}
	return gsc
}

// Initialize connects to the WebSocket, fetches initial runtime and metadata.
func (gsc *GoSidecar) Initialize(blockID int) (err error) {
	websocket.SetEndpoint(gsc.wsUrl)
	if gsc.printTraces {
		log.Println("Fetching initial runtime version...")
	}
	blockHash, err := rpc.GetChainGetBlockHash(nil, blockID)
	if err != nil {
		return fmt.Errorf("failed to get block %d hash for runtime version fetch: %w", blockID, err)
	}
	if gsc.printTraces {
		log.Printf("[Block %d] Fetching runtime version for %s", blockID, blockHash)
	}
	runtime, err := getRuntime(blockID, blockHash)
	if err != nil {
		return err
	}
	if gsc.printTraces {
		log.Printf("[Block %d] Initial Spec Version %d obtained.", blockID, runtime.SpecVersion)
	}
	gsc.runtimes["relay-chain"] = runtime
	if gsc.printTraces {
		log.Printf("[Block %d] Fetching metadata for spec %d using hash %s", blockID, runtime.SpecVersion, blockHash)
	}
	meta, err := getMetadata(runtime.SpecVersion, blockHash)
	if err != nil {
		return err
	}
	gsc.metadatas[runtime.SpecVersion] = meta
	if gsc.printTraces {
		log.Printf("[Block %d] Metadata for spec %d successfully loaded.", blockID, runtime.SpecVersion)
	}
	return nil
}

func (gsc *GoSidecar) Close() {
	websocket.Close()
}

func (gsc *GoSidecar) GetDecodedBlock(blockNum int) (block *dix.BlockData, err error) {
	// HASH
	hash, err := gsc.getBlockHash(blockNum)
	if err != nil {
		return nil, fmt.Errorf("error getting block hash for %d: %w", blockNum, err)
	}
	// EXTRINSICS
	encodedBlock, _, err := gsc.fetchBlockDetails(hash, blockNum)
	if err != nil {
		return nil, fmt.Errorf("error fetching block details for %d (%s): %w", blockNum, hash, err)
	}
	// EVENTS
	encodedEvents, _, err := gsc.fetchEvents(hash, blockNum)
	if err != nil {
		return nil, fmt.Errorf("error fetching events for block %d (%s): %w", blockNum, hash, err)
	}
	// RUNTIME
	runtimeInfo, ok := gsc.runtimes["relay-chain"] // TODO: Make chain name dynamic or configurable
	if !ok {
		return nil, fmt.Errorf("runtime info for 'relay-chain' not found for block %d", blockNum)
	}
	// METADATA
	meta, ok := gsc.metadatas[runtimeInfo.SpecVersion]
	if !ok {
		return nil, fmt.Errorf("metadata for spec version %d not found for block %d", runtimeInfo.SpecVersion, blockNum)
	}
	// Decode extrinsics
	extrinsics, err := gsc.decodeExtrinsics(blockNum, encodedBlock.Block.Extrinsics, meta, runtimeInfo.SpecVersion)
	if err != nil {
		return nil, err
	}
	// Decode events
	events, err := gsc.decodeEvents(blockNum, encodedEvents, meta, runtimeInfo.SpecVersion)
	if err != nil {
		return nil, err
	}
	// map events per module
	var eventsSet map[string][]int
	for e := range events {
		key, ok := events[e]["call_module"].(string)
		if ok {
			eventsSet[key] = append(eventsSet[key], e)
		}
	}

	// Assign to the named return variable 'block'
	block = &dix.BlockData{}
	block.ID = strconv.Itoa(blockNum)
	block.Hash = hash
	block.ParentHash = encodedBlock.Block.Header.ParentHash
	block.StateRoot = encodedBlock.Block.Header.StateRoot
	block.ExtrinsicsRoot = encodedBlock.Block.Header.ExtrinsicsRoot
	block.Logs = json.RawMessage("[]")
	block.Extrinsics = json.RawMessage("[]")
	block.AuthorID = ""
	block.Finalized = false
	block.OnInitialize = nil
	block.OnFinalize = nil

	// update Logs
	storageLogs, err := substrate.DecodeLogDigest(encodedBlock.Block.Header.Digest.Logs)
	if err == nil {
		var digest BlockDigest
		for l, logItem := range storageLogs {
			logValue := interface2array(logItem.Value)
			digest.Logs = append(digest.Logs, &BlockLog{
				Type: logItem.Type,
				Index: l,
				Value: logValue,
			})
			// log.Printf("DEBUG %d %v %v", l, logItem.Type, logValue)
			if logItem.Type == "PreRuntime" {
				validators, err := getValidators(hash)
				// log.Printf("DEBUG Found %d validators", len(validators))
				if err == nil {
					author := substrate.ExtractAuthor([]byte(interface2string(logItem.Value)), validators)
					block.AuthorID = ss58.Encode(author, 42)
				}
				// log.Printf("DEBUG Author: %v", block.AuthorID)
			}
		}
		block.Logs, err = json.Marshal(digest)
		// log.Printf("%v", block.Logs)
	}

	// extract data from block
	for e, extrinsic := range extrinsics {
		extrinsicName := fmt.Sprintf("%d-%d", blockNum, e)
		e_call_module, ok := extrinsic["call_module"]
		if !ok {
			return nil, fmt.Errorf("[Extrinsic %s] Warning: no call module", extrinsicName)
		}
		call_module, ok := e_call_module.(string)
		if !ok {
			return nil, fmt.Errorf("[Extrinsic %s] Warning: call module is not a string: %v", extrinsicName, err)
		}
		switch call_module {
		case "Timestamp":
			blockTimestamp, err := extractTimestamp(extrinsicName, extrinsic)
			if err != nil {
				log.Printf("[Extrinsic %s] Warning: Cannot find a timestamp: %v", extrinsicName, err)
			} else {
				block.Timestamp = blockTimestamp
			}
		default:
			if gsc.printTraces {
				log.Printf("[Extrinsic %s] %s", extrinsicName, call_module)
			}
		}
		// era / mortal
		encodedEra, ok := extrinsic["era"].(string)
		if !ok {
			encodedEra = ""
		}
		mortal := substrate.DecodeMortal(encodedEra)
		delete(extrinsic, "era")
		extrinsic["era"], err = json.Marshal(mortal)
		// merge events
		eventIndexes, ok := eventsSet[call_module]
		if ok && len(eventIndexes) > 0 {
			var relevant []Event
			for i := range eventIndexes {
				relevant = append(relevant, events[eventIndexes[i]])
			}
			extrinsic["events"], err = json.Marshal(relevant)
		}
	}

	// debug loop
	if gsc.printDebug {
		for e, extrinsic := range extrinsics {
			json, err := json.MarshalIndent(extrinsic, "", "  ")
			if err != nil {
				log.Printf("Block %d ERROR Extrinsic %d: keys are %s", blockNum, e, slices.Sorted(maps.Keys(extrinsic)))
				continue
			}
			log.Printf("[Extrinsic %d-%d] %s", blockNum, e, json)
		}
	}

	block.Extrinsics, err = json.Marshal(extrinsics)
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (gsc *GoSidecar) getBlockHash(blockNum int) (string, error) {
	if gsc.printTraces {
		log.Printf("[Block %d] Fetching initial hash (for runtime/metadata)...", blockNum)
	}
	hash, err := rpc.GetChainGetBlockHash(nil, blockNum)
	if err != nil {
		return "", fmt.Errorf("[Block %d] Failed to get initial block hash: %v", blockNum, err)
	}
	if hash == "" {
		log.Printf("[Block %d] Received empty initial block hash", blockNum)
		return "", nil
	}
	return hash, nil
}

func (gsc *GoSidecar) fetchBlockDetails(blockHash string, blockNum int) (blockResponse EncodedBlock, downloadDuration time.Duration, err error) {
	if gsc.printTraces {
		log.Printf("[Block %d] Fetching details hash=%s", blockNum, blockHash)
	}
	downloadBlockStart := time.Now()
	blockRequest := rpc.ChainGetBlock(rand.Intn(10000), blockHash)
	var rpcBlockResult model.JsonRpcResult
	err = websocket.SendWsRequest(nil, &rpcBlockResult, blockRequest)
	downloadDuration = time.Since(downloadBlockStart)
	if err != nil {
		return EncodedBlock{}, downloadDuration, fmt.Errorf("failed to send block request for block %d (%s): %w", blockNum, blockHash, err)
	}
	if rpcBlockResult.Error != nil {
		return EncodedBlock{}, downloadDuration, fmt.Errorf("RPC error fetching block %d (%s): %v", blockNum, blockHash, rpcBlockResult.Error)
	}
	if rpcBlockResult.Result == nil {
		return EncodedBlock{}, downloadDuration, nil
	}
	result, ok  := rpcBlockResult.Result.(map[string]interface{})
	if !ok {
		log.Printf("ERROR: [Block %d] rpcResult is not a map", blockNum)
		return EncodedBlock{}, downloadDuration, nil // Not an error, just no data
	}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return EncodedBlock{}, downloadDuration, fmt.Errorf("[Block %d] failed to marshal block result interface (%s): %w", blockNum, blockHash, err)
	}
	err = json.Unmarshal(resultBytes, &blockResponse)
	if err != nil {
		log.Printf("[Block %d] Failed to unmarshal block result from JSON (%s): %v. Skipping block.", blockNum, blockHash, err)
		return EncodedBlock{}, downloadDuration, nil
	}
	if gsc.printTraces {
		log.Printf("[Block %d] details fetched. Found %d extrinsics.", blockNum, len(blockResponse.Block.Extrinsics))
	}
	return blockResponse, downloadDuration, nil
}

func (gsc *GoSidecar) fetchEvents(blockHash string, blockNum int) (string, time.Duration, error) {
	if gsc.printTraces {
		log.Printf("[Block %d] Fetching events storage for hash %s", blockNum, blockHash)
	}
	downloadEventsStart := time.Now()
	var rawEventsHex string                // Declare rawEventsHex outside the if block
	var rpcEventResult model.JsonRpcResult // Declare rpcEventResult here

	eventsKeyBytes := storageKey.EncodeStorageKey("System", "Events")

	storageRequest := rpc.StateGetStorage(
		rand.Intn(10000),
		rpcutil.AddHex(eventsKeyBytes.EncodeKey),
		blockHash)
	err := websocket.SendWsRequest(nil, &rpcEventResult, storageRequest)
	downloadEventsDuration := time.Since(downloadEventsStart)

	if err != nil {
		return "", downloadEventsDuration, fmt.Errorf("[Block %d] failed to send event storage request (%s): %w", blockNum, blockHash, err)
	}
	if rpcEventResult.Error != nil {
		return "", downloadEventsDuration, fmt.Errorf("[Block %d] RPC error fetching events (%s): %v", blockNum, blockHash, rpcEventResult.Error)
	}
	if rpcEventResult.Result == nil {
		log.Printf("[Block %d] Received nil result for events storage (%s). No events found?", blockNum, blockHash)
		return "", downloadEventsDuration, nil
	}

	// Attempt to extract the hex string result
	resultBytes, err := json.Marshal(rpcEventResult.Result)
	if err != nil {
		return "", downloadEventsDuration, fmt.Errorf("[Block %d] failed to marshal event storage result interface (%s): %w", blockNum, blockHash, err)
	}
	err = json.Unmarshal(resultBytes, &rawEventsHex)
	if err != nil {
		// Treat unmarshal error as potentially empty events, but log a warning
		log.Printf("[Block %d] Failed to unmarshal event storage result from JSON (%s): %v. Assuming empty events.", blockNum, blockHash, err)
		return "", downloadEventsDuration, nil
	}

	if gsc.printTraces {
		log.Printf("[Block %d] Events storage fetched in %v", blockNum, downloadEventsDuration)
	}
	return rawEventsHex, downloadEventsDuration, nil
}

// decodeExtrinsics performs sequential decoding of extrinsics.
func (gsc *GoSidecar) decodeExtrinsics(
	blockNum int,
	extrinsics []string,
	meta *metadata.Instant,
	specVersion int,
) ([]map[string]interface{}, error) {
	if gsc.printTraces {
		log.Printf("[Block %d] Starting decoding of %d extrinsics...", blockNum, len(extrinsics))
	}
	decodedExtrinsicData, err := substrate.DecodeExtrinsic(extrinsics, meta, specVersion)
	if err != nil {
		return nil, fmt.Errorf("[Block %d] ERROR: Failed to decode extrinsics: %v", blockNum, err)
	}
	return decodedExtrinsicData, nil
}

// decodeEvents performs sequential decoding of events.
func (gsc *GoSidecar) decodeEvents(
	blockNum int, // Added for logging context
	rawEventsHex string,
	meta *metadata.Instant,
	specVersion int,
) ([]map[string]interface{}, error) { // Return type changed
	if rawEventsHex == "" || rawEventsHex == "0x" {
		return nil, fmt.Errorf("[Block %d] Skipping event decoding as rawEventsHex is empty.", blockNum)
	}

	decodedEvents, err := substrate.DecodeEvent(rawEventsHex, meta, specVersion)
	if err != nil {
		return nil, fmt.Errorf("[Block %d] Decoding events failed %v.", blockNum, err)
	}
	eventsInterface, ok := decodedEvents.([]interface{})
	if !ok {
		return nil, fmt.Errorf("Decoded events are not a slice, cannot iterate. Type: %T", decodedEvents)
	}
	// debug loop
	if gsc.printDebug {
		for e, event := range eventsInterface {
			json, err := json.MarshalIndent(event, "", "  ")
			if err != nil {
				continue
			}
			log.Printf("[Event %d-%d] %s", blockNum, e, json)
		}
	}
	var events []map[string]interface{}
	for i, e := range eventsInterface {
		ems, ok := e.(map[string]interface{})
		if !ok {
			log.Printf("[Event %d-%d] Warning: Event casting failed", blockNum, i)
		}
		events = append(events, ems)
	}
	return events, nil
}

func main() {
	wsURL := flag.String("ws", "", "WebSocket endpoint URL (required)")
	startBlockNum := flag.Int("start", 0, "Start block number")
	blockCount := flag.Int("count", 1, "Number of blocks to process")
	printOutput := flag.Bool("print", false, "Print decoded extrinsics and events")
	printDebug := flag.Bool("debug", false, "Print debug information")
	printTraces := flag.Bool("traces", false, "Print traces")

	flag.Parse()

	if *wsURL == "" {
		fmt.Println("WebSocket URL (-ws) is required")
		flag.Usage()
		return
	}

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Create and initialize GoSidecar
	gsc := NewGoSidecar(*wsURL, *printTraces, *printDebug, *printOutput)
	if err := gsc.Initialize(*startBlockNum); err != nil {
		log.Fatalf("Failed to initialize GoSidecar: %v", err)
	}
	defer gsc.Close()

	// Process blocks
	failedBlocks := 0
	for i := 0; i < *blockCount; i++ {
		blockNum := *startBlockNum + i
		block, err := gsc.GetDecodedBlock(blockNum)
		if err != nil {
			log.Printf("ERROR: Failed to get block %d: %v", blockNum, err)
			failedBlocks++
			continue
		}
		if *printOutput {
			jsBlock, err := json.MarshalIndent(block, "", "  ")
			if err != nil {
				log.Printf("ERROR: Failed to marshal block %d: %v", blockNum, err)
				failedBlocks++
				continue
			}
			log.Printf("%v", jsBlock)
		}
	}

	log.Printf("Processed %d blocks, failed %d blocks", *blockCount-failedBlocks, failedBlocks)
}
