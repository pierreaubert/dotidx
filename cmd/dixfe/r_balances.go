package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	dix "github.com/pierreaubert/dotidx"
)

func (f *Frontend) handleBalances(w http.ResponseWriter, r *http.Request) {
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

	count := query.Get("count")
	log.Printf("Count: %s", count)
	if count == "" {
		count = "10"
	}
	log.Printf("Count: %s", count)

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
	blocks, err := f.getBlocksByAddress(address, count, fromTimestamp, toTimestamp)
	if err != nil {
		log.Printf("Error getting blocks for address %s: %v", address, err)
		http.Error(w, "Failed to retrieve blocks", http.StatusInternalServerError)
		return
	}

	eb := dix.NewEventsBalance(address)
	for block := range blocks {
		filtered, err := eb.Process(blocks[block].Extrinsics)
		if err != nil {
			http.Error(w, "Failed to extract balances from block", http.StatusInternalServerError)
			return
		}
		blocks[block].OnInitialize = []byte("[]")
		blocks[block].OnFinalize = []byte("[]")
		blocks[block].Logs = []byte("[]")
		blocks[block].Extrinsics = filtered
	}

	// Set content type header
	w.Header().Set("Content-Type", "application/json")

	// Return response
	json.NewEncoder(w).Encode(blocks)
}
