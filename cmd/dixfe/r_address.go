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

	count := r.URL.Query().Get("count")
	if count == "" {
		count = "10"
	}

	from := r.URL.Query().Get("from")
	var fromTimestamp string
	if from == "" {
		fromTimestamp = ""
	} else {
		// Try to parse the from parameter as a timestamp
		fromTime, err := dotidx.ParseTimestamp(from)
		if err != nil {
			http.Error(w, "Invalid 'from' timestamp format", http.StatusBadRequest)
			return
		}
		// Format as SQL timestamp
		fromTimestamp = fromTime.Format("2006-01-02 15:04:05")
	}

	to := r.URL.Query().Get("to")
	var toTimestamp string
	if to == "" {
		toTimestamp = ""
	} else {
		// Try to parse the to parameter as a timestamp
		toTime, err := dotidx.ParseTimestamp(to)
		if err != nil {
			http.Error(w, "Invalid 'to' timestamp format", http.StatusBadRequest)
			return
		}
		// Format as SQL timestamp
		toTimestamp = toTime.Format("2006-01-02 15:04:05")
	}

	if !dotidx.IsValidAddress(address) {
		http.Error(w, "Invalid address format", http.StatusBadRequest)
		return
	}

	blocks, err := f.getBlocksByAddress(address, count, fromTimestamp, toTimestamp)
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

func (f *Frontend) getBlocksByAddress(address string, count, from, to string) ([]dotidx.BlockData, error) {
	if !dotidx.IsValidAddress(address) {
		return nil, fmt.Errorf("invalid address format")
	}

	cond := ""
	if from != "" {
		cond = fmt.Sprintf("AND b.created_at >= '%s'", from)
	}
	if to != "" {
		if cond != "" {
			cond += fmt.Sprintf(" AND b.created_at <= '%s'", to)
		} else {
			cond = fmt.Sprintf("AND b.created_at <= '%s'", to)
		}
	}

	query := fmt.Sprintf(
		`SELECT * FROM (SELECT b.*
		FROM %s b
		JOIN %s a ON b.block_id = a.block_id
		WHERE a.address = '%s'
		%s
		ORDER BY b.block_id DESC
		LIMIT %s) ORDER BY block_id ASC;`,
		dotidx.GetBlocksTableName(f.config),
		dotidx.GetAddressTableName(f.config),
		address,
		cond,
		count,
	)
	log.Printf("Query: %s", query)
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
