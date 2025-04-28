package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/itering/substrate-api-rpc"
	"github.com/itering/substrate-api-rpc/metadata"
	"github.com/itering/substrate-api-rpc/model"
	rpc "github.com/itering/substrate-api-rpc/rpc"
	rpcutil "github.com/itering/substrate-api-rpc/util"
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

// --- Worker Pool Types ---
type extrinsicTask struct {
	index       int
	extrinsic   string
	meta        interface{} // Use interface{} to bypass specific type resolution issues
	specVersion int
}

type extrinsicResult struct {
	index            int
	decodedExtrinsic interface{} // Use interface{} for broader compatibility
	err              error
	duration         time.Duration // Time taken by the worker for this task
}

// --- Structs for Event Decoding Worker Pool ---
type eventTask struct {
	rawEventsHex string      // Raw storage data for events (hex string)
	meta         interface{} // Use interface{} for flexibility, assert in worker
	specVersion  int
}

type eventResult struct {
	decodedEvents interface{} // Result from DecodeEvent (generic type)
	err           error
	duration      time.Duration // Time taken by the worker for this task
}

// --- Worker Function ---
func extrinsicWorker(id int, wg *sync.WaitGroup, tasks <-chan extrinsicTask, results chan<- extrinsicResult) {
	defer wg.Done()
	log.Printf("[Worker %d] Started", id)
	for task := range tasks {
		// log.Printf("[Worker %d] Decoding extrinsic %d", id, task.index)

		// Type assertion for metadata
		metaTyped, ok := task.meta.(*metadata.Instant)
		if !ok {
			log.Printf("[Worker %d] ERROR: Invalid metadata type received for extrinsic %d. Expected *metadata.Instant, got %T", id, task.index, task.meta)
			results <- extrinsicResult{
				index:            task.index,
				decodedExtrinsic: nil,
				err:              fmt.Errorf("invalid metadata type: %T", task.meta),
				duration:         0,
			}
			continue // Skip this task
		}

		startTime := time.Now()
		decoded, err := substrate.DecodeExtrinsic([]string{task.extrinsic}, metaTyped, task.specVersion)
		duration := time.Since(startTime)
		results <- extrinsicResult{
			index:            task.index,
			decodedExtrinsic: decoded,
			err:              err,
			duration:         duration,
		}
	}
	log.Printf("[Worker %d] Finished", id)
}

// --- Event Worker Function ---
func eventWorker(id int, wg *sync.WaitGroup, tasks <-chan eventTask, results chan<- eventResult) {
	defer wg.Done()
	// log.Printf("[EventWorker %d] Started", id)
	for task := range tasks {
		// log.Printf("[EventWorker %d] Decoding event hex...", id)

		// Type assertion for metadata
		metaTyped, ok := task.meta.(*metadata.Instant)
		if !ok {
			log.Printf("[EventWorker %d] ERROR: Invalid metadata type received. Expected *metadata.Instant, got %T", id, task.meta)
			results <- eventResult{
				decodedEvents: nil,
				err:           fmt.Errorf("invalid metadata type: %T", task.meta),
				duration:      0,
			}
			continue // Skip this task
		}

		startTime := time.Now()
		// Use DecodeEvent based on original sequential code
		decoded, err := substrate.DecodeEvent(task.rawEventsHex, metaTyped, task.specVersion)
		duration := time.Since(startTime)

		results <- eventResult{
			decodedEvents: decoded,
			err:           err,
			duration:      duration,
		}
		// log.Printf("[EventWorker %d] Finished decoding event hex in %v", id, duration)
	}
	// log.Printf("[EventWorker %d] Stopping", id)
}

func main() {
	wsUrl := flag.String("ws", "", "WebSocket endpoint URL (required)")
	startBlockNum := flag.Int("block", -1, "Starting block number (required)")
	blockCount := flag.Int("count", 1, "Number of blocks to fetch and decode")
	printOutput := flag.Bool("print", true, "Print decoded extrinsics and events")

	flag.Parse()

	if *wsUrl == "" {
		fmt.Println("Error: -ws flag (WebSocket endpoint URL) is required.")
		flag.Usage()
		os.Exit(1)
	}
	if *startBlockNum < 0 {
		fmt.Println("Error: -block flag (Starting block number) is required and must be non-negative.")
		flag.Usage()
		os.Exit(1)
	}
	if *blockCount <= 0 {
		fmt.Println("Error: -count flag must be positive.")
		flag.Usage()
		os.Exit(1)
	}

	// Initialize WebSocket connection
	log.Printf("Setting endpoint to %s...", *wsUrl)
	websocket.SetEndpoint(*wsUrl)
	log.Println("Endpoint set.")

	// --- 1. Fetch Initial Block Hash (for Runtime/Metadata) ---
	log.Printf("Fetching initial hash for block %d (for runtime/metadata)...", *startBlockNum)
	initialBlockHash, err := rpc.GetChainGetBlockHash(nil, *startBlockNum)
	if err != nil {
		log.Fatalf("Failed to get initial block hash for block %d: %v", *startBlockNum, err)
	}
	if initialBlockHash == "" {
		log.Fatalf("Received empty initial block hash for block %d", *startBlockNum)
	}
	log.Printf("Initial Block %d Hash: %s", *startBlockNum, initialBlockHash)

	// --- 2. Get Runtime Version (One-off, using initial block hash) ---
	log.Println("Fetching runtime version...")
	var rpcRuntimeResult model.JsonRpcResult
	var runtimeVersionResult RuntimeVersion
	runtimeRequest := rpc.StateGetRuntimeVersion(rand.Intn(10000), initialBlockHash)
	err = websocket.SendWsRequest(nil, &rpcRuntimeResult, runtimeRequest)
	if err != nil {
		log.Fatalf("Failed to send/receive runtime version request for hash %s: %v", initialBlockHash, err)
	}
	if rpcRuntimeResult.Error != nil {
		log.Fatalf("RPC error fetching runtime version for hash %s: %v", initialBlockHash, rpcRuntimeResult.Error)
	}
	if rpcRuntimeResult.Result == nil {
		log.Fatalf("Received nil result for runtime version for hash %s", initialBlockHash)
	}
	resultBytes, err := json.Marshal(rpcRuntimeResult.Result)
	if err != nil {
		log.Fatalf("Failed to marshal runtime version result interface: %v", err)
	}
	err = json.Unmarshal(resultBytes, &runtimeVersionResult)
	if err != nil {
		log.Fatalf("Failed to unmarshal runtime version result from JSON: %v", err)
	}
	specVersion := runtimeVersionResult.SpecVersion
	if specVersion == 0 {
		log.Printf("WARN: Received runtime version with specVersion 0 for hash %s: %+v", initialBlockHash, runtimeVersionResult)
	}
	log.Printf("Using Spec Version %d for all blocks in this run.", specVersion)

	// --- 3. Get Metadata (One-off, using initial block hash) ---
	log.Println("Fetching metadata...")
	rawMetadataString, err := rpc.GetMetadataByHash(nil, initialBlockHash)
	if err != nil {
		log.Fatalf("Failed to get metadata by hash %s: %v", initialBlockHash, err)
	}
	if rawMetadataString == "" {
		log.Fatalf("Received empty metadata for hash %s", initialBlockHash)
	}
	log.Println("Decoding metadata...")
	meta := metadata.RegNewMetadataType(specVersion, rawMetadataString)
	if meta == nil {
		log.Fatalf("Failed to process metadata for hash %s", initialBlockHash)
	}
	log.Println("Metadata decoded successfully.")

        // Adjust as needed, e.g., runtime.NumCPU()
	numWorkers := 4

	// --- Initialize Worker Pool ---
	extrinsicTasks := make(chan extrinsicTask, 100) // Buffered channel
	extrinsicResults := make(chan extrinsicResult, 100)
	var workerWg sync.WaitGroup
	log.Printf("Starting %d extrinsic decoder workers...", numWorkers)
	for w := 1; w <= numWorkers; w++ {
		workerWg.Add(1)
		go extrinsicWorker(w, &workerWg, extrinsicTasks, extrinsicResults)
	}

	// --- Initialize Event Worker Pool ---
	eventTasks := make(chan eventTask, 100) // Buffered channel
	eventResults := make(chan eventResult, 100)
	var eventWorkerWg sync.WaitGroup
	log.Printf("Starting %d event decoder workers...", numWorkers)
	for w := 1; w <= numWorkers; w++ {
		eventWorkerWg.Add(1)
		go eventWorker(w, &eventWorkerWg, eventTasks, eventResults)
	}

	// --- BENCHMARK LOOP START ---
	log.Printf("Starting benchmark for %d block(s) from block %d...", *blockCount, *startBlockNum)
	loopStartTime := time.Now() // Overall loop start time

	var durations []time.Duration // Store durations for each block
	var totalTime time.Duration   // Total processing time for all blocks
	var worstTime time.Duration   // Worst processing time for a single block
	processedCount := 0           // Counter for successfully processed blocks

	// Detailed timing accumulators
	var totalDownloadBlockTime time.Duration
	var totalDecodeExtrinsicsPhaseTime time.Duration
	var totalDownloadEventsTime time.Duration
	var totalDecodeEventsTime time.Duration

	// Slices to store individual durations for percentile calculation
	var downloadBlockTimes []time.Duration
	var decodeExtrinsicsPhaseTimes []time.Duration  // Duration of the parallel phase
	var decodeExtrinsicsWorkerTimes []time.Duration // Individual worker task durations
	var downloadEventsTimes []time.Duration
	var decodeEventsPhaseTimes []time.Duration // Store event phase durations
	var eventWorkerTimes []time.Duration       // Store individual event worker durations

	for i := 0; i < *blockCount; i++ {
		currentBlockNum := *startBlockNum + i
		log.Printf("--- Processing Block %d --- (Spec Version: %d)", currentBlockNum, specVersion)
		blockStartTime := time.Now() // Start timer for this block

		// --- 4. Get Block Hash --- (Needed for subsequent calls)
		blockHash, err := rpc.GetChainGetBlockHash(nil, currentBlockNum)
		if err != nil {
			log.Printf("ERROR: Failed to get block hash for block %d: %v. Skipping block.", currentBlockNum, err)
			continue // Skip to the next block
		}
		if blockHash == "" {
			log.Printf("ERROR: Received empty block hash for block %d. Skipping block.", currentBlockNum)
			continue // Skip to the next block
		}
		log.Printf("Block %d Hash: %s", currentBlockNum, blockHash)

		// --- 5. Fetch Block Details (including Extrinsics) ---
		log.Println("Fetching block details...")
		downloadBlockStart := time.Now()
		var blockResponse BlockResponse
		blockRequest := rpc.ChainGetBlock(rand.Intn(10000), blockHash)
		var rpcBlockResult model.JsonRpcResult
		err = websocket.SendWsRequest(nil, &rpcBlockResult, blockRequest)
		downloadBlockDuration := time.Since(downloadBlockStart)
		totalDownloadBlockTime += downloadBlockDuration
		downloadBlockTimes = append(downloadBlockTimes, downloadBlockDuration)
		if err != nil {
			log.Printf("ERROR: Failed to send block request for block %d (%s): %v. Skipping block.", currentBlockNum, blockHash, err)
			continue // Skip to the next block
		}
		if rpcBlockResult.Error != nil {
			log.Printf("ERROR: RPC error fetching block %d (%s): %v. Skipping block.", currentBlockNum, blockHash, rpcBlockResult.Error)
			continue // Skip to the next block
		}
		if rpcBlockResult.Result == nil {
			log.Printf("ERROR: Received nil result for block %d (%s). Skipping block.", currentBlockNum, blockHash)
			continue // Skip to the next block
		}
		resultBytes, err := json.Marshal(rpcBlockResult.Result)
		if err != nil {
			log.Printf("ERROR: Failed to marshal block result interface for block %d (%s): %v. Skipping block.", currentBlockNum, blockHash, err)
			continue
		}
		err = json.Unmarshal(resultBytes, &blockResponse)
		if err != nil {
			log.Printf("ERROR: Failed to unmarshal block result from JSON for block %d (%s): %v. Skipping block.", currentBlockNum, blockHash, err)
			continue
		}
		log.Printf("Block %d details fetched. Found %d extrinsics.", currentBlockNum, len(blockResponse.Block.Extrinsics))

		// --- Print Header and Justifications (if requested) ---
		if *printOutput {
			fmt.Printf("\n--- Block %d Header ---\n", currentBlockNum)
			headerJSON, err := json.MarshalIndent(blockResponse.Block.Header, "", "  ")
			if err != nil {
				log.Printf("WARN: Failed to marshal block header to JSON: %v\n", err)
				fmt.Printf("  Raw Header Data: %+v\n", blockResponse.Block.Header)
			} else {
				fmt.Println(string(headerJSON))
			}

			// --- Decode and Print Digest Logs ---
			headerMap, ok := blockResponse.Block.Header.(map[string]interface{})
			if !ok {
				log.Printf("WARN: Could not assert Header as map[string]interface{} to decode logs.")
			} else {
				digestInterface, digestExists := headerMap["digest"]
				if !digestExists {
					log.Println("INFO: No 'digest' field found in header.")
				} else {
					digestMap, ok := digestInterface.(map[string]interface{})
					if !ok {
						log.Printf("WARN: Could not assert digest as map[string]interface{} to decode logs.")
					} else {
						logsInterface, logsExist := digestMap["logs"]
						if !logsExist {
							log.Println("INFO: No 'logs' field found in header.digest.")
						} else {
							logsSlice, ok := logsInterface.([]interface{})
							if !ok {
								log.Printf("WARN: Could not assert header.digest.logs as []interface{}.")
							} else {
								fmt.Printf("\n--- Block %d Digest Logs (%d) ---\n", currentBlockNum, len(logsSlice))
								logStrings := make([]string, 0, len(logsSlice))
								for i, logItem := range logsSlice {
									logString, ok := logItem.(string)
									if !ok {
										log.Printf("WARN: Log item %d is not a string (%T), skipping.", i, logItem)
										continue
									}
									logStrings = append(logStrings, logString)
								}

								if len(logStrings) > 0 {
									decodedLogs, err := substrate.DecodeLogDigest(logStrings)
									if err != nil {
										log.Printf("WARN: Failed to decode digest logs: %v\n", err)
										fmt.Println("--- Digest Logs (Decode Error) ---")
										// Optionally print raw strings on error
										for i, s := range logStrings {
											fmt.Printf("  [%d]: %s\n", i, s)
										}
									} else {
										fmt.Println("--- Digest Logs (Decoded) ---")
										logsJSON, err := json.MarshalIndent(decodedLogs, "", "  ")
										if err != nil {
											log.Printf("WARN: Failed to marshal decoded logs to JSON: %v\n", err)
											fmt.Printf("  Raw Decoded Logs Data: %+v\n", decodedLogs)
										} else {
											fmt.Println(string(logsJSON))
										}
									}
								}
							}
						}
					}
				}
			}

			if blockResponse.Justifications != nil {
				fmt.Printf("\n--- Block %d Justifications ---\n", currentBlockNum)
				justificationsJSON, err := json.MarshalIndent(blockResponse.Justifications, "", "  ")
				if err != nil {
					log.Printf("WARN: Failed to marshal block justifications to JSON: %v\n", err)
					fmt.Printf("  Raw Justifications Data: %+v\n", blockResponse.Justifications)
				} else {
					fmt.Println(string(justificationsJSON))
				}
			}
		}

		// --- 6. Decode Extrinsics (Parallel) ---
		decodeExtrinsicsPhaseStart := time.Now()
		numExtrinsics := len(blockResponse.Block.Extrinsics)
		collectedExtrinsicResults := make([]extrinsicResult, numExtrinsics) // Store results in order
		var collectorWg sync.WaitGroup

		if numExtrinsics > 0 {
			collectorWg.Add(1) // Add 1 for the collector goroutine
			// Goroutine to collect results
			go func() {
				defer collectorWg.Done()
				for i := 0; i < numExtrinsics; i++ {
					result := <-extrinsicResults
					if result.index >= 0 && result.index < numExtrinsics { // Basic bounds check
						collectedExtrinsicResults[result.index] = result
					} else {
						log.Printf("ERROR: Received result with out-of-bounds index %d for block %d", result.index, currentBlockNum)
					}
				}
			}()

			// Submit tasks
			if *printOutput {
				fmt.Printf("\n--- Submitting %d extrinsics for decoding (Block %d, Spec %d) ---\n", numExtrinsics, currentBlockNum, specVersion)
			}
			for idx, extrinsic := range blockResponse.Block.Extrinsics {
				task := extrinsicTask{
					index:       idx,
					extrinsic:   extrinsic,
					meta:        meta,
					specVersion: specVersion,
				}
				extrinsicTasks <- task
			}

			// Wait for the collector to finish collecting all results for this block
			collectorWg.Wait()

			// Process collected results
			for _, result := range collectedExtrinsicResults {
				decodeExtrinsicsWorkerTimes = append(decodeExtrinsicsWorkerTimes, result.duration) // Collect worker time
				if result.err != nil {
					log.Printf("WARN: Failed to decode extrinsic %d: %v\n", result.index, result.err)
					if *printOutput {
						// We don't have the raw extrinsic string here easily, might need to pass it in result if needed
						fmt.Printf("  Extrinsic %d decoding failed.\n", result.index)
					}
					continue
				}
				if *printOutput {
					fmt.Printf("--- Extrinsic %d (Decoded) ---\n", result.index)
					prettyJSON, err := json.MarshalIndent(result.decodedExtrinsic, "", "  ")
					if err != nil {
						log.Printf("WARN: Failed to marshal extrinsic %d to JSON: %v\n", result.index, err)
						fmt.Printf("  Raw decoded data: %+v\n", result.decodedExtrinsic)
					} else {
						fmt.Println(string(prettyJSON))
					}
				}
			}
		} // end if numExtrinsics > 0

		decodeExtrinsicsPhaseEnd := time.Now()
		decodeExtrinsicsPhaseTime := decodeExtrinsicsPhaseEnd.Sub(decodeExtrinsicsPhaseStart)
		totalDecodeExtrinsicsPhaseTime += decodeExtrinsicsPhaseTime // Accumulate phase time
		decodeExtrinsicsPhaseTimes = append(decodeExtrinsicsPhaseTimes, decodeExtrinsicsPhaseTime)

		if *printOutput {
			fmt.Println("\n--- Done with extrinsics --- ")
		}

		// --- 7. Download Events --- NOTE: Restoring original fetching logic
		downloadEventsStart := time.Now()
		var rawEventsHex string                // Declare rawEventsHex outside the if block
		var rpcEventResult model.JsonRpcResult // Declare rpcEventResult here

		eventsKeyBytes := storageKey.EncodeStorageKey("System", "Events")

		storageRequest := rpc.StateGetStorage(
			rand.Intn(10000),
			rpcutil.AddHex(eventsKeyBytes.EncodeKey),
			blockHash)
		err = websocket.SendWsRequest(nil, &rpcEventResult, storageRequest)

		if err != nil {
			log.Printf("WARN: Failed to send event storage request for block %d (%s): %v", currentBlockNum, blockHash, err)
			// Continue, but rawEventsHex will be empty, skipping decoding
		} else if rpcEventResult.Error != nil {
			log.Printf("WARN: RPC error fetching events for block %d (%s): %v", currentBlockNum, blockHash, rpcEventResult.Error)
			// Continue, but rawEventsHex will be empty
		} else if rpcEventResult.Result == nil {
			log.Printf("INFO: Received nil result for events storage for block %d (%s). No events found?", currentBlockNum, blockHash)
			// rawEventsHex remains empty
		} else {
			// Attempt to extract the hex string result
			resultBytes, err := json.Marshal(rpcEventResult.Result)
			if err != nil {
				log.Printf("WARN: Failed to marshal event storage result interface for block %d (%s): %v", currentBlockNum, blockHash, err)
			} else {
				err = json.Unmarshal(resultBytes, &rawEventsHex)
				if err != nil {
					log.Printf("WARN: Failed to unmarshal event storage result from JSON for block %d (%s): %v", currentBlockNum, blockHash, err)
					rawEventsHex = "" // Ensure it's empty on error
				}
			}
		}
		downloadEventsEnd := time.Now()
		downloadEventsDuration := downloadEventsEnd.Sub(downloadEventsStart)
		totalDownloadEventsTime += downloadEventsDuration
		downloadEventsTimes = append(downloadEventsTimes, downloadEventsDuration)

		// --- 8. Decode Events (Parallel) ---
		if rawEventsHex != "" && rawEventsHex != "0x" {
			decodeEventsPhaseStart := time.Now()

			// Submit event task to worker pool
			task := eventTask{
				rawEventsHex: rawEventsHex,
				meta:         meta,
				specVersion:  specVersion,
			}
			eventTasks <- task

			// Wait for event worker result
			result := <-eventResults

			decodeEventsPhaseEnd := time.Now()
			decodeEventsPhaseTime := decodeEventsPhaseEnd.Sub(decodeEventsPhaseStart)
			totalDecodeEventsTime += decodeEventsPhaseTime // Accumulate phase time
			decodeEventsPhaseTimes = append(decodeEventsPhaseTimes, decodeEventsPhaseTime)

			if result.err == nil {
				// Append successful worker duration
				eventWorkerTimes = append(eventWorkerTimes, result.duration)
			} else {
				log.Printf("ERROR: Failed to decode events for block %d (%s): %v", currentBlockNum, blockHash, result.err)
			}

			if *printOutput && result.err == nil {
				fmt.Printf("\n--- Events for Block %d ---\n", currentBlockNum)
				eventsJSON, err := json.MarshalIndent(result.decodedEvents, "", "  ")
				if err != nil {
					log.Printf("WARN: Failed to marshal events to JSON: %v\n", err)
					fmt.Printf("  Raw Events Data: %+v\n", result.decodedEvents)
				} else {
					fmt.Println(string(eventsJSON))
				}
			}
		} else {
			// Log if skipping because rawEventsHex is empty (unless already logged above)
			if err == nil && rpcEventResult.Error == nil && rpcEventResult.Result != nil { // Only log if fetch seemed ok but result was empty
				log.Printf("INFO: Skipping event decoding for block %d (%s) as no event data found in storage.", currentBlockNum, blockHash)
			}
		}

		// --- End of Block Processing ---
		blockEndTime := time.Now()                   // Stop timer for this block
		duration := blockEndTime.Sub(blockStartTime) // Calculate duration
		durations = append(durations, duration)      // Store duration
		totalTime += duration                        // Add to total time
		if duration > worstTime {
			worstTime = duration // Update worst time
		}
		processedCount++ // Increment successfully processed count
		log.Printf("Block %d processed in %v", currentBlockNum, duration)

	} // --- END OF BENCHMARK LOOP ---

	loopEndTime := time.Now()
	// overallDuration := loopEndTime.Sub(loopStartTime) // Removed unused variable

	// --- Calculate and Print Summary ---
	if processedCount > 0 {
		totalTime := loopEndTime.Sub(loopStartTime)
		avgBlockTime := totalTime / time.Duration(processedCount)

		fmt.Println("\n--- Final Performance Summary ---")
		fmt.Printf("Total Time: %v\n", totalTime)
		fmt.Printf("Total Blocks Processed: %d\n", processedCount)
		fmt.Printf("Avg Block Processing Time: %v\n", avgBlockTime)

		avgDownloadBlock := calculateAverageDurationFromSlice(downloadBlockTimes)
		avgDecodeExtrinsicsPhase := calculateAverageDurationFromSlice(decodeExtrinsicsPhaseTimes)
		avgDownloadEvents := calculateAverageDurationFromSlice(downloadEventsTimes)
		avgDecodeEventsPhase := calculateAverageDurationFromSlice(decodeEventsPhaseTimes)
		sumOfAverages := avgDownloadBlock + avgDecodeExtrinsicsPhase + avgDownloadEvents + avgDecodeEventsPhase

		fmt.Println("\n--- Average Phase Durations ---")
		fmt.Printf("Avg Download Block Time: %v\n", avgDownloadBlock)
		fmt.Printf("Avg Extrinsic Decode Phase Time: %v\n", avgDecodeExtrinsicsPhase)
		fmt.Printf("Avg Download Events Time: %v\n", avgDownloadEvents)
		fmt.Printf("Avg Event Decode Phase Time: %v\n", avgDecodeEventsPhase)
		fmt.Printf("P99 Download Block Time: %v\n", calculatePercentile(downloadBlockTimes, 99))
		fmt.Printf("P99 Extrinsic Decode Phase Time: %v\n", calculatePercentile(decodeExtrinsicsPhaseTimes, 99))
		fmt.Printf("P99 Download Events Time: %v\n", calculatePercentile(downloadEventsTimes, 99))
		fmt.Printf("P99 Event Decode Phase Time: %v\n", calculatePercentile(decodeEventsPhaseTimes, 99))
		fmt.Printf("  Sum of Avg Phases: %v (vs Avg Block Time: %v)\n", sumOfAverages, avgBlockTime)
		fmt.Println("(Note: Sum of phase averages might not exactly equal average block time due to overhead between phases)")

		fmt.Printf("\n--- Average Worker Durations ---")
		fmt.Printf("Avg Extrinsic Decode Worker Time: %v\n", calculateAverageDurationFromSlice(decodeExtrinsicsWorkerTimes))
		fmt.Printf("P99 Extrinsic Decode Worker Time: %v\n", calculatePercentile(decodeExtrinsicsWorkerTimes, 99))
		fmt.Printf("Avg Event Decode Worker Time: %v\n", calculateAverageDurationFromSlice(eventWorkerTimes))
		fmt.Printf("P99 Event Decode Worker Time: %v\n", calculatePercentile(eventWorkerTimes, 99))

	} else {
		fmt.Println("No blocks were successfully processed.")
	}

	// --- Shutdown Worker Pool ---
	log.Println("Closing extrinsic task channel...")
	close(extrinsicTasks) // Signal workers to stop
	log.Println("Waiting for workers to finish...")
	workerWg.Wait()         // Wait for all workers to exit
	close(extrinsicResults) // Close results channel after workers are done
	log.Println("Workers finished.")

	// --- Shutdown Event Worker Pool ---
	log.Println("Closing event task channel...")
	close(eventTasks) // Signal workers to stop
	log.Println("Waiting for event workers to finish...")
	eventWorkerWg.Wait() // Wait for all workers to exit
	close(eventResults)  // Close results channel after workers are done
	log.Println("Event workers finished.")
}

func calculateAverageDurationFromSlice(durations []time.Duration) time.Duration {
	var total time.Duration
	if len(durations) == 0 {
		return 0
	}
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

func calculatePercentile(durations []time.Duration, percentile int) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	// Create a copy to avoid modifying the original slice
	sortedDurations := make([]time.Duration, len(durations))
	copy(sortedDurations, durations)
	sort.Slice(sortedDurations, func(i, j int) bool {
		return sortedDurations[i] < sortedDurations[j]
	})
	pIndex := int(float64(len(sortedDurations))*float64(percentile)/100.0) - 1
	if pIndex < 0 {
		pIndex = 0 // Handle small sample sizes
	}
	if pIndex >= len(sortedDurations) { // Ensure index is within bounds
		pIndex = len(sortedDurations) - 1
	}
	return sortedDurations[pIndex]
}
