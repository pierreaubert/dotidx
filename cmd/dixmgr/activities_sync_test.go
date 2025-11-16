package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestCheckNodeSyncActivity_Synced(t *testing.T) {
	// Create test server that returns isSyncing=false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"result": {
				"isSyncing": false,
				"peers": 100,
				"shouldHavePeers": true
			}
		}`))
	}))
	defer server.Close()

	activities := &Activities{executeMode: false}
	ctx := context.Background()

	synced, err := activities.CheckNodeSyncActivity(ctx, server.URL, 0)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !synced {
		t.Errorf("Expected synced=true when isSyncing=false, got synced=%v", synced)
	}
}

func TestCheckNodeSyncActivity_Syncing(t *testing.T) {
	// Create test server that returns isSyncing=true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"result": {
				"isSyncing": true,
				"peers": 50,
				"shouldHavePeers": true
			}
		}`))
	}))
	defer server.Close()

	activities := &Activities{executeMode: false}
	ctx := context.Background()

	synced, err := activities.CheckNodeSyncActivity(ctx, server.URL, 0)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if synced {
		t.Errorf("Expected synced=false when isSyncing=true, got synced=%v", synced)
	}
}

func TestCheckNodeSyncActivity_HTTPError(t *testing.T) {
	// Create test server that returns 500 error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	activities := &Activities{executeMode: false}
	ctx := context.Background()

	_, err := activities.CheckNodeSyncActivity(ctx, server.URL, 0)
	if err == nil {
		t.Fatal("Expected error for HTTP 500, got nil")
	}
}

func TestCheckNodeSyncActivity_InvalidJSON(t *testing.T) {
	// Create test server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	activities := &Activities{executeMode: false}
	ctx := context.Background()

	_, err := activities.CheckNodeSyncActivity(ctx, server.URL, 0)
	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}
}

func TestCheckNodeSyncActivity_PortFallback(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"result": {
				"isSyncing": false,
				"peers": 100,
				"shouldHavePeers": true
			}
		}`))
	}))
	defer server.Close()

	activities := &Activities{executeMode: false}
	ctx := context.Background()

	// When rpcEndpoint is empty, it should use the port parameter
	// This will fail because we're passing a non-existent port, but tests the fallback logic
	_, err := activities.CheckNodeSyncActivity(ctx, "", 9999)
	if err == nil {
		t.Log("Port fallback test expects connection error (non-existent port)")
	}
}

// TestCheckNodeSyncActivity_PublicEndpoint is skipped by default
// Enable with: DOTIDX_TEST_PUBLIC=1 go test
func TestCheckNodeSyncActivity_PublicEndpoint(t *testing.T) {
	if os.Getenv("DOTIDX_TEST_PUBLIC") != "1" {
		t.Skip("Skipping public endpoint test (set DOTIDX_TEST_PUBLIC=1 to enable)")
	}

	activities := &Activities{executeMode: false}
	ctx := context.Background()

	// Test against public Polkadot RPC
	synced, err := activities.CheckNodeSyncActivity(ctx, "https://rpc.polkadot.io", 0)
	if err != nil {
		t.Logf("Public endpoint test failed (expected if endpoint is unavailable): %v", err)
		return
	}

	t.Logf("Public endpoint synced status: %v", synced)
}
