package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"math/rand"

	substrate "github.com/itering/substrate-api-rpc"
	"github.com/itering/substrate-api-rpc/metadata"
	"github.com/itering/substrate-api-rpc/model"
	"github.com/itering/substrate-api-rpc/rpc"
	"github.com/itering/substrate-api-rpc/storageKey"
	"github.com/itering/substrate-api-rpc/websocket"
)

// Define structs based on typical JSON RPC responses if not provided by the library
type RuntimeVersion struct {
	SpecName           string `json:"specName"`
	ImplName           string `json:"implName"`
	AuthoringVersion   int    `json:"authoringVersion"`
	SpecVersion        int    `json:"specVersion"`
	ImplVersion        int    `json:"implVersion"`
	TransactionVersion int    `json:"transactionVersion"`
	StateVersion       int    `json:"stateVersion"`
}

type BlockResponse struct {
	Block struct {
		Header     interface{} `json:"header"` // Can define Header struct if needed
		Extrinsics []string    `json:"extrinsics"`
	} `json:"block"`
	Justifications interface{} `json:"justifications"` // Optional
}

func main() {
	ws := flag.String("ws", "", "Parachain RPC endpoint URL")
	blockNumStr := flag.String("block", "", "Block number to fetch (required)")
	requestId := flag.Int("id", 1, "JSON RPC request ID (default: 1)") // Default to 1
	flag.Parse()

	if *ws == "" || *blockNumStr == "" {
		fmt.Fprintf(os.Stderr, "Both -ws and -block parameters are required\n")
		flag.Usage()
		os.Exit(1)
	}

	blockNum, err := strconv.Atoi(*blockNumStr)
	if err != nil {
		log.Fatalf("Invalid block number: %v", err)
	}

	// --- 0. Set Endpoint --- Use SetEndpoint for the global pool
	log.Printf("Setting endpoint to %s...\n", *ws)
	websocket.SetEndpoint(*ws)
	log.Println("Endpoint set.")

	// --- 1. Get Block Hash --- Use Get... function with nil connection
	log.Printf("Fetching hash for block %d...\n", blockNum)
	blockHash, err := rpc.GetChainGetBlockHash(nil, blockNum) // Pass nil for WsConn
	if err != nil {
		log.Fatalf("Failed to get block hash for block %d: %v", blockNum, err)
	}
	if blockHash == "" {
		log.Fatalf("Received empty block hash for block %d", blockNum)
	}
	log.Printf("Block %d Hash: %s\n", blockNum, blockHash)

	// --- 2. Get Runtime Version (using SendWsRequest with nil connection and intermediate decoding) ---
	log.Println("Fetching runtime version...")
	var rpcRuntimeResult model.JsonRpcResult // Intermediate result holder
	var runtimeVersionResult RuntimeVersion  // Final result holder
	runtimeVersionRequest := rpc.ChainGetRuntimeVersion(*requestId, blockHash)
	err = websocket.SendWsRequest(nil, &rpcRuntimeResult, runtimeVersionRequest) // Decode into JsonRpcResult
	if err != nil {
		log.Fatalf("Failed to send/receive runtime version request for hash %s: %v", blockHash, err)
	}
	if rpcRuntimeResult.Error != nil {
		log.Fatalf("RPC error fetching runtime version for hash %s: %v", blockHash, rpcRuntimeResult.Error)
	}
	if rpcRuntimeResult.Result == nil {
		log.Fatalf("Received nil result for runtime version for hash %s", blockHash)
	}
	// Marshal the Result interface{} back to JSON
	resultBytes, err := json.Marshal(rpcRuntimeResult.Result)
	if err != nil {
		log.Fatalf("Failed to marshal runtime version result interface: %v", err)
	}
	// Unmarshal the JSON into the final RuntimeVersion struct
	err = json.Unmarshal(resultBytes, &runtimeVersionResult)
	if err != nil {
		log.Fatalf("Failed to unmarshal runtime version result from JSON: %v", err)
	}

	specVersion := runtimeVersionResult.SpecVersion
	if specVersion == 0 {
		log.Printf("WARN: Received runtime version with specVersion 0 for hash %s: %+v", blockHash, runtimeVersionResult)
		// Allow proceeding, maybe it's valid for some chains/blocks
	}
	log.Printf("Spec Version at block %d: %d\n", blockNum, specVersion)

	// --- 3. Get Metadata (using Get... function with nil connection) ---
	log.Println("Fetching metadata...")
	rawMetadataString, err := rpc.GetMetadataByHash(nil, blockHash) // Pass nil for WsConn
	if err != nil {
		log.Fatalf("Failed to get metadata by hash %s: %v", blockHash, err)
	}
	if rawMetadataString == "" {
		log.Fatalf("Received empty metadata for hash %s", blockHash)
	}
	meta := metadata.RegNewMetadataType(specVersion, rawMetadataString)
	log.Println("Metadata processed.")

	// --- 4. Get Block Data (using SendWsRequest with nil connection and intermediate decoding) ---
	log.Println("Fetching block data...")
	var rpcBlockResult model.JsonRpcResult // Intermediate result holder
	var blockResult BlockResponse          // Final result holder
	blockRequest := rpc.ChainGetBlock(*requestId, blockHash)
	err = websocket.SendWsRequest(nil, &rpcBlockResult, blockRequest) // Decode into JsonRpcResult
	if err != nil {
		log.Fatalf("Failed to send/receive block data request for hash %s: %v", blockHash, err)
	}
	if rpcBlockResult.Error != nil {
		log.Fatalf("RPC error fetching block data for hash %s: %v", blockHash, rpcBlockResult.Error)
	}
	if rpcBlockResult.Result == nil {
		log.Fatalf("Received nil result for block data for hash %s", blockHash)
	}
	// Marshal the Result interface{} back to JSON
	resultBytes, err = json.Marshal(rpcBlockResult.Result)
	if err != nil {
		log.Fatalf("Failed to marshal block data result interface: %v", err)
	}
	// Unmarshal the JSON into the final BlockResponse struct
	err = json.Unmarshal(resultBytes, &blockResult)
	if err != nil {
		log.Fatalf("Failed to unmarshal block data result from JSON: %v", err)
	}

	log.Printf("Block data fetched successfully. Found %d extrinsics.\n", len(blockResult.Block.Extrinsics))

	// --- 5. Decode Extrinsics ---
	if blockResult.Block.Extrinsics == nil || len(blockResult.Block.Extrinsics) == 0 {
		log.Printf("No extrinsics found in block %d.", blockNum)
		fmt.Println("--- Done ---")
		return // Exit cleanly
	}

	fmt.Printf("\n--- Decoding %d extrinsics for block %d (Spec Version: %d) ---\n", len(blockResult.Block.Extrinsics), blockNum, specVersion)
	for i, extrinsic := range blockResult.Block.Extrinsics {
		decodedExtrinsic, err := substrate.DecodeExtrinsic([]string{extrinsic}, meta, specVersion)
		if err != nil {
			log.Printf("WARN: Failed to decode extrinsic %d: %v\n", i, err)
			fmt.Printf("  Raw Extrinsic %d: %s\n", i, extrinsic)
			continue
		}
		// Pretty print the decoded extrinsic using JSON marshaling
		fmt.Printf("--- Extrinsic %d ---\n", i)
		prettyJSON, err := json.MarshalIndent(decodedExtrinsic, "", "  ") // Use 2 spaces for indentation
		if err != nil {
			log.Printf("WARN: Failed to marshal extrinsic %d to JSON: %v\n", i, err)
			fmt.Printf("  Raw decoded data: %+v\n", decodedExtrinsic) // Fallback to default print
		} else {
			fmt.Println(string(prettyJSON))
		}
	}
	fmt.Println("\n--- Done with extrinsics ---")

	// --- 6. Fetch and Decode Events ---
	log.Println("Fetching events...")
	// Generate storage key for System.Events
	eventsKey := storageKey.EncodeStorageKey("System", "Events") // Corrected: Only returns the key struct
	// No error check needed here as EncodeStorageKey doesn't return an error

	// Prepare RPC request for state_getStorageAt
	storageRequest := rpc.StateGetStorage(rand.Intn(10000), eventsKey.EncodeKey, blockHash) // Corrected: Randomize ID
	var rpcEventResult model.JsonRpcResult

	// Send request
	err = websocket.SendWsRequest(nil, &rpcEventResult, storageRequest)
	if err != nil {
		log.Fatalf("Failed to send event storage request: %v", err)
	}
	if rpcEventResult.Error != nil {
		log.Fatalf("RPC error fetching events: %v", rpcEventResult.Error)
	}
	if rpcEventResult.Result == nil {
		log.Printf("WARN: Received nil result for events storage for block %s. No events found?", blockHash)
	} else {
		// Intermediate decode: Marshal Result to bytes, then Unmarshal to string
		resultBytes, err = json.Marshal(rpcEventResult.Result)
		if err != nil {
			log.Fatalf("Failed to marshal event storage result interface: %v", err)
		}
		var rawEventsHex string
		err = json.Unmarshal(resultBytes, &rawEventsHex)
		if err != nil {
			log.Fatalf("Failed to unmarshal event storage result from JSON: %v", err)
		}

		if rawEventsHex == "" || rawEventsHex == "0x" {
			log.Println("No events found in storage for this block.")
		} else {
			// Decode the raw event data
			decodedEvents, err := substrate.DecodeEvent(rawEventsHex, meta, specVersion)
			if err != nil {
				log.Fatalf("Failed to decode events: %v\nRaw Events Hex: %s", err, rawEventsHex)
			}

			// Type assert to a slice for iteration
			eventsSlice, ok := decodedEvents.([]interface{}) // Assume it's a slice
			if !ok {
				log.Fatalf("Decoded events are not a slice, cannot iterate. Type: %T", decodedEvents)
			}

			fmt.Printf("\n--- Decoding %d events for block %d (Spec Version: %d) ---\n", len(eventsSlice), blockNum, specVersion)
			for i, eventRecord := range eventsSlice {
				fmt.Printf("--- Event %d ---\n", i)
				prettyJSON, err := json.MarshalIndent(eventRecord, "", "  ")
				if err != nil {
					log.Printf("WARN: Failed to marshal event %d to JSON: %v\n", i, err)
					fmt.Printf("  Raw decoded event: %+v\n", eventRecord)
				} else {
					fmt.Println(string(prettyJSON))
				}
			}
		}
	}

	fmt.Println("\n--- Done ---")
}
