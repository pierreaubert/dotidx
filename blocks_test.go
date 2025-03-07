package main

//goland:noinspection Annotator,Annotator,Annotator,Annotator,Annotator,Annotator,Annotator,Annotator
import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

func TestExtractAddressesFromExtrinsics(t *testing.T) {
	tests := []struct {
		name       string
		extrinsics string
		expected   int
		err        bool
	}{
		{
			name:       "Empty extrinsics",
			extrinsics: `[]`,
			expected:   0,
			err:        false,
		},
		{
			name:       "Invalid JSON",
			extrinsics: `invalid`,
			expected:   0,
			err:        true,
		},
		{
			name:       "ID field",
			extrinsics: `[{"id": "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY"}]`,
			expected:   1,
			err:        false,
		},
		{
			name:       "Multiple ID fields",
			extrinsics: `[{"id": "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY"}, {"user_id": "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty"}]`,
			expected:   2,
			err:        false,
		},
		{
			name:       "Data array with Polkadot addresses",
			extrinsics: `[{"data": ["5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY", "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty"]}]`,
			expected:   2,
			err:        false,
		},
		{
			name:       "Nested data array",
			extrinsics: `[{"nested": {"data": ["5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY", "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty"]}}]`,
			expected:   2,
			err:        false,
		},
		{
			name:       "Combined ID and data fields",
			extrinsics: `[{"id": "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY", "data": ["5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty"]}]`,
			expected:   2,
			err:        false,
		},
		{
			name:       "Duplicate addresses",
			extrinsics: `[{"id": "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY"}, {"data": ["5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY", "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty"]}]`,
			expected:   2,
			err:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addresses, err := extractAddressesFromExtrinsics(json.RawMessage(tt.extrinsics))
			if tt.err {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(addresses) != tt.expected {
					t.Errorf("extractAddressesFromExtrinsics() got %d addresses, expected %d", len(addresses), tt.expected)
				}
			}
		})
	}
}

func TestExtractAddressesFromRealData(t *testing.T) {
	// Get all JSON files in the tests/data/blocks directory
	blockDir := "tests/data/blocks"
	files, err := os.ReadDir(blockDir)
	if err != nil {
		t.Fatalf("Failed to read blocks directory: %v", err)
	}

	// Filter for JSON files
	jsonFiles := make([]string, 0)
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			jsonFiles = append(jsonFiles, filepath.Join(blockDir, file.Name()))
		}
	}

	if len(jsonFiles) == 0 {
		t.Fatalf("No JSON files found in %s", blockDir)
	}

	t.Logf("Found %d JSON files to test", len(jsonFiles))

	// Process each JSON file
	for _, jsonFile := range jsonFiles {
		t.Run(jsonFile, func(t *testing.T) {
			// Read the file
			fileData, err := os.ReadFile(jsonFile)
			if err != nil {
				t.Fatalf("Failed to read file %s: %v", jsonFile, err)
			}

			// Parse the JSON to extract the extrinsics field
			var blockData struct {
				Extrinsics json.RawMessage `json:"extrinsics"`
			}
			if err := json.Unmarshal(fileData, &blockData); err != nil {
				t.Fatalf("Failed to unmarshal JSON from %s: %v", jsonFile, err)
			}

			// Extract addresses from the extrinsics
			addresses, err := extractAddressesFromExtrinsics(blockData.Extrinsics)
			if err != nil {
				t.Logf("Error extracting addresses from %s: %v", jsonFile, err)
				return
			}

			// Log the extracted addresses
			t.Logf("Extracted %d addresses from %s", len(addresses), jsonFile)
			for i, addr := range addresses {
				t.Logf("  Address %d: %s", i+1, addr)
			}

			// Count Polkadot addresses
			polkadotAddresses := len(addresses)
			t.Logf("Found %d Polkadot addresses in %s", polkadotAddresses, jsonFile)

			// Verify that all addresses start with a valid prefix (typically 1-9 or A-Z)
			for _, addr := range addresses {
				if strings.HasPrefix(addr, "0x") {
					t.Errorf("Found hex address %s in %s, expected only Polkadot addresses", addr, jsonFile)
				}
			}
		})
	}
}
