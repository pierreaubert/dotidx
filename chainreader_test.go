package dotidx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestFetchHeadBlock(t *testing.T) {
	// Create a test server with a handler that returns a mock head block
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/blocks/head" {
			t.Errorf("Expected request to /blocks/head, got %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Return a mock head block response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Create a mock block with ID 12345
		mockBlock := BlockData{
			ID:             "12345",
			Timestamp:      time.Now(),
			Hash:           "0xabcdef1234567890",
			ParentHash:     "0x1234567890abcdef",
			StateRoot:      "0xabcdef1234567890",
			ExtrinsicsRoot: "0x1234567890abcdef",
			AuthorID:       "0xabcdef1234",
			Finalized:      true,
			OnInitialize:   json.RawMessage("{}"),
			OnFinalize:     json.RawMessage("{}"),
			Logs:           json.RawMessage("{}"),
			Extrinsics:     json.RawMessage("{}"),
		}

		json.NewEncoder(w).Encode(mockBlock)
	}))
	defer server.Close()

	reader := NewSidecar(server.URL)

	// Call the fetchHeadBlock function with the test server URL
	headBlockID, err := reader.GetChainHeadID()
	if err != nil {
		t.Fatalf("fetchHeadBlock returned an error: %v", err)
	}

	// Verify the returned head block ID
	expectedID := 12345
	if headBlockID != expectedID {
		t.Errorf("Expected head block ID %d, got %d", expectedID, headBlockID)
	}
}

func TestCallSidecar(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the request URL matches the expected pattern
		if r.URL.Path != "/blocks/1" {
			t.Errorf("Expected request to '/blocks/1', got '%s' with query params %v", r.URL.Path, r.URL.Query())
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		// Return a mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{
			"number": "1",
			"timestamp": "2023-01-01T00:00:00Z",
			"hash": "0x1234567890abcdef",
			"parentHash": "0xabcdef1234567890",
			"stateRoot": "0x1234567890abcdef1234567890abcdef",
			"extrinsicsRoot": "0xabcdef1234567890abcdef1234567890",
			"authorId": "0x1234567890",
			"finalized": true,
			"onInitialize": {},
			"onFinalize": {},
			"logs": [],
			"extrinsics": []
		}`)
	}))
	defer server.Close()

	// Create a context
	ctx := context.Background()

	// Call the function
	reader := NewSidecar(server.URL)
	blockData, err := reader.FetchBlock(ctx, 1)
	if err != nil {
		t.Fatalf("fetchData returned an error: %v", err)
	}

	// Check the result
	if blockData.ID != "1" {
		t.Errorf("Expected ID=1, got %s", blockData.ID)
	}
	if blockData.Hash != "0x1234567890abcdef" {
		t.Errorf("Expected Hash=0x1234567890abcdef, got %s", blockData.Hash)
	}
	if blockData.ParentHash != "0xabcdef1234567890" {
		t.Errorf("Expected ParentHash=0xabcdef1234567890, got %s", blockData.ParentHash)
	}
	if !blockData.Finalized {
		t.Errorf("Expected Finalized=true, got %v", blockData.Finalized)
	}
}

func TestFetchBlockRange(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the request URL matches the expected pattern for block range
		if r.URL.Path != "/blocks" {
			t.Errorf("Expected request to '/blocks', got '%s'", r.URL.Path)
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		// Check query parameters
		query := r.URL.Query()
		rangeParam := query.Get("range")

		if rangeParam != "100-105" {
			t.Errorf("Expected query parameter range=100-105, got range=%s", rangeParam)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Return a mock response with multiple blocks
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return an array of blocks
		fmt.Fprintln(w, `[
			{
				"number": "100",
				"timestamp": "2023-01-01T00:00:00Z",
				"hash": "0x1234567890abcdef1",
				"parentHash": "0xabcdef1234567890",
				"stateRoot": "0x1234567890abcdef1234567890abcdef",
				"extrinsicsRoot": "0xabcdef1234567890abcdef1234567890",
				"authorId": "0x1234567890",
				"finalized": true,
				"onInitialize": {},
				"onFinalize": {},
				"logs": [],
				"extrinsics": []
			},
			{
				"number": "101",
				"timestamp": "2023-01-01T00:01:00Z",
				"hash": "0x1234567890abcdef2",
				"parentHash": "0x1234567890abcdef1",
				"stateRoot": "0x1234567890abcdef1234567890abcdef",
				"extrinsicsRoot": "0xabcdef1234567890abcdef1234567890",
				"authorId": "0x1234567890",
				"finalized": true,
				"onInitialize": {},
				"onFinalize": {},
				"logs": [],
				"extrinsics": []
			},
			{
				"number": "102",
				"timestamp": "2023-01-01T00:02:00Z",
				"hash": "0x1234567890abcdef3",
				"parentHash": "0x1234567890abcdef2",
				"stateRoot": "0x1234567890abcdef1234567890abcdef",
				"extrinsicsRoot": "0xabcdef1234567890abcdef1234567890",
				"authorId": "0x1234567890",
				"finalized": true,
				"onInitialize": {},
				"onFinalize": {},
				"logs": [],
				"extrinsics": []
			}
		]`)
	}))
	defer server.Close()

	// Create a context
	ctx := context.Background()

	// Create an array of block IDs to fetch
	blockIDs := []int{100, 101, 102, 103, 104, 105}

	reader := NewSidecar(server.URL)

	// Call the function to fetch blocks with the specified IDs
	blocks, err := reader.FetchBlockRange(ctx, blockIDs)
	if err != nil {
		t.Fatalf("fetchBlockRange returned an error: %v", err)
	}

	// Check that we got the expected number of blocks
	if len(blocks) != 3 {
		t.Fatalf("Expected 3 blocks, got %d", len(blocks))
	}

	// Check the first block
	if blocks[0].ID != "100" {
		t.Errorf("Expected first block ID=100, got %s", blocks[0].ID)
	}
	if blocks[0].Hash != "0x1234567890abcdef1" {
		t.Errorf("Expected first block Hash=0x1234567890abcdef1, got %s", blocks[0].Hash)
	}

	// Check the second block
	if blocks[1].ID != "101" {
		t.Errorf("Expected second block ID=101, got %s", blocks[1].ID)
	}
	if blocks[1].Hash != "0x1234567890abcdef2" {
		t.Errorf("Expected second block Hash=0x1234567890abcdef2, got %s", blocks[1].Hash)
	}

	// Check the third block
	if blocks[2].ID != "102" {
		t.Errorf("Expected third block ID=102, got %s", blocks[2].ID)
	}
	if blocks[2].Hash != "0x1234567890abcdef3" {
		t.Errorf("Expected third block Hash=0x1234567890abcdef3, got %s", blocks[2].Hash)
	}
}
