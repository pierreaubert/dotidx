package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/pierreaubert/dotidx"
)

func (f *Frontend) handleBalances(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the address from the query parameters
	address := r.URL.Query().Get("address")
	if address == "" {
		http.Error(w, "Missing address parameter", http.StatusBadRequest)
		return
	}

	// Check if the address is a valid Polkadot address
	if !dotidx.IsValidAddress(address) {
		http.Error(w, "Invalid address format", http.StatusBadRequest)
		return
	}

	// Retrieve blocks for this address using the existing function
	blocks, err := f.getBlocksByAddress(address)
	if err != nil {
		log.Printf("Error getting blocks for address %s: %v", address, err)
		http.Error(w, "Failed to retrieve blocks", http.StatusInternalServerError)
		return
	}

	eb := dotidx.NewEventsBalance(address)
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
