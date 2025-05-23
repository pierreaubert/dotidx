package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/pierreaubert/dotidx/dix"
)

func (f *Frontend) handleStaking(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the address from the query parameters
	query := r.URL.Query()
	address := query.Get("address")
	if address == "" {
		http.Error(w, "Missing address parameter", http.StatusBadRequest)
		return
	}

	// Check if the address is a valid Polkadot address
	if !dix.IsValidAddress(address) {
		http.Error(w, "Invalid address format", http.StatusBadRequest)
		return
	}

	from := query.Get("from")
	var fromTimestamp string
	if from == "" {
		fromTimestamp = ""
	} else {
		// Try to parse the from parameter as a timestamp
		fromTime, err := dix.ParseTimestamp(from)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid 'from' %s timestamp format", from), http.StatusBadRequest)
			return
		}
		// Format as SQL timestamp
		fromTimestamp = fromTime.Format("2006-01-02 15:04:05.0000")
	}

	to := query.Get("to")
	var toTimestamp string
	if to == "" {
		toTimestamp = ""
	} else {
		// Try to parse the to parameter as a timestamp
		toTime, err := dix.ParseTimestamp(to)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid 'to' %s timestamp format", to), http.StatusBadRequest)
			return
		}
		// Format as SQL timestamp
		toTimestamp = toTime.Format("2006-01-02 15:04:05.0000")
	}

	// Retrieve blocks for this address using the existing function
	count := "5000"
	blocks, err := f.getBlocksByAddress(address, count, fromTimestamp, toTimestamp)
	if err != nil {
		log.Printf("Error getting blocks for address %s: %v", address, err)
		http.Error(w, "Failed to retrieve blocks", http.StatusInternalServerError)
		return
	}

	es := dix.NewEventsStaking(address)
	filteredBlocks := make(map[string]map[string][]dix.BlockData)
	for relay := range blocks {
		filteredBlocks[relay] = make(map[string][]dix.BlockData)
		for chain := range blocks[relay] {
			filteredBlocks[relay][chain] = make([]dix.BlockData, 0)
			for block := range blocks[relay][chain] {
				iblock := blocks[relay][chain][block]
				filtered, found, err := es.Process(iblock.Extrinsics)
				if err != nil {
					log.Printf("Failed to extract stakings from block %s on chain %s:%s: %v",
						iblock.ID, relay, chain, err)
					return
				}
				if found {
					log.Printf("Extracted staking events from block %s on chain %s:%s Timestamp %s", iblock.ID, relay, chain, iblock.Timestamp)
					fblock := &dix.BlockData{
						ID:         iblock.ID,
						Timestamp:  iblock.Timestamp,
						Hash:       iblock.Hash,
						ParentHash: iblock.ParentHash,
						Extrinsics: filtered,
					}
					filteredBlocks[relay][chain] = append(filteredBlocks[relay][chain], *fblock)
				}
			}
		}
	}

	// Set content type header
	w.Header().Set("Content-Type", "application/json")

	// Return response
	json.NewEncoder(w).Encode(filteredBlocks)
}
