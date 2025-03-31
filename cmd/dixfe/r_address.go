package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	dix "github.com/pierreaubert/dotidx"
)

type BlocksResponse struct {
	Blocks []dix.BlockData `json:"blocks"`
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
		fromTime, err := dix.ParseTimestamp(from)
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
		toTime, err := dix.ParseTimestamp(to)
		if err != nil {
			http.Error(w, "Invalid 'to' timestamp format", http.StatusBadRequest)
			return
		}
		// Format as SQL timestamp
		toTimestamp = toTime.Format("2006-01-02 15:04:05")
	}

	if !dix.IsValidAddress(address) {
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

func (f *Frontend) getBlocksByAddressForChain(relay, chain, address string, count, from, to string) ([]dix.BlockData, error) {
	if !dix.IsValidAddress(address) {
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
		dix.GetBlocksTableName(relay, chain),
		dix.GetAddressTableName(relay, chain),
		address,
		cond,
		count,
	)
	rows, err := f.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}
	defer rows.Close()

	log.Printf("Query: %s", query)

	var blocks []dix.BlockData

	for rows.Next() {
		var block dix.BlockData
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

func (f *Frontend) getBlocksByAddress(address string, count, from, to string) (
	map[string]map[string][]dix.BlockData,
	error,
) {
	blocks := make(map[string]map[string][]dix.BlockData)
	var wg sync.WaitGroup
	var err error

	for relay := range f.config.Parachains {
		blocks[relay] = make(map[string][]dix.BlockData)
		for chain := range f.config.Parachains[relay] {
			wg.Add(1)
			go func() {
				defer wg.Done()
				blocks[relay][chain], err = f.getBlocksByAddressForChain(relay, chain, address, count, from, to)
				if err != nil {
					log.Printf("Error getting blocks for address %s: %v", address, err)
				}
			}()
		}
	}
	wg.Wait()
	return blocks, nil
}
