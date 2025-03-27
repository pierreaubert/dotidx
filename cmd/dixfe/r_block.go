package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	dix "github.com/pierreaubert/dotidx"
)

func (f *Frontend) handleBlock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("blockid")
	block, err := f.getBlock(id)
	if err != nil {
		log.Printf("Error getting block for id %s: %v", id, err)
		http.Error(w, "Error retrieving a block", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(block); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

func (f *Frontend) getBlock(id string) (dix.BlockData, error) {
	query := fmt.Sprintf(`SELECT * FROM %s WHERE block_id = %s;`, dix.GetBlocksTableName(f.config), id)
	var block dix.BlockData
	if err := f.db.QueryRow(query).Scan(
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
	); err != nil {
		if err == sql.ErrNoRows {
			return block, fmt.Errorf("no block with %s", id)
		}
		return block, fmt.Errorf("Cant scan block %s: %v", id, err)
	}
	return block, nil
}
