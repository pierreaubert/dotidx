package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/pierreaubert/dotidx"
)

type BlocksResponse struct {
	Blocks []dotidx.BlockData `json:"blocks"`
}

func (f *Frontend) handleAddressToBlocks(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	defer func() {
		f.metricsHandler.RecordLatency(startTime, http.StatusOK, nil)
	}()

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	address := r.URL.Query().Get("address")
	if address == "" {
		http.Error(w, "Missing address parameter", http.StatusBadRequest)
		return
	}

	if !dotidx.IsValidAddress(address) {
		http.Error(w, "Invalid address format", http.StatusBadRequest)
		return
	}

	blocks, err := f.getBlocksByAddress(address)
	if err != nil {
		log.Printf("Error getting blocks for address %s: %v", address, err)
		http.Error(w, "Error retrieving blocks", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(blocks); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

func (f *Frontend) getBlocksByAddress(address string) ([]dotidx.BlockData, error) {
	if !dotidx.IsValidAddress(address) {
		return nil, fmt.Errorf("invalid address format")
	}

	query := fmt.Sprintf(
		`SELECT
			b.block_id,
			b.created_at,
			b.hash,
			b.parent_hash,
			b.state_root,
			b.extrinsics_root,
			b.author_id,
			b.finalized,
			b.on_initialize,
			b.on_finalize,
			b.logs,
			b.extrinsics
		FROM %s b
		JOIN %s a ON b.block_id = a.block_id
		WHERE a.address = '%s'
		ORDER BY b.block_id DESC
		LIMIT 10;`,
		dotidx.GetBlocksTableName(f.config),
		dotidx.GetAddressTableName(f.config),
		address,
	)

	rows, err := f.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}
	defer rows.Close()

	var blocks []dotidx.BlockData

	for rows.Next() {
		var block dotidx.BlockData
		err = rows.Scan(
			&block.ID,
			&block.Timestamp,
			&block.Hash,
			&block.ParentHash,
			&block.StateRoot,
			&block.ExtrinsicsRoot,
			&block.AuthorID,
			&block.Finalized,
			&block.OnInitialize,
			&block.OnFinalize,
			&block.Logs,
			&block.Extrinsics,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning block: %w", err)
		}

		blocks = append(blocks, block)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating blocks: %w", err)
	}

	return blocks, nil
}
