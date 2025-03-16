package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type BlockData struct {
	ID             string          `json:"number"`
	Timestamp      time.Time       `json:"timestamp"`
	Hash           string          `json:"hash"`
	ParentHash     string          `json:"parentHash"`
	StateRoot      string          `json:"stateRoot"`
	ExtrinsicsRoot string          `json:"extrinsicsRoot"`
	AuthorID       string          `json:"authorId"`
	Finalized      bool            `json:"finalized"`
	OnInitialize   json.RawMessage `json:"onInitialize"`
	OnFinalize     json.RawMessage `json:"onFinalize"`
	Logs           json.RawMessage `json:"logs"`
	Extrinsics     json.RawMessage `json:"extrinsics"`
}

func extractTimestamp(extrinsics []byte) (ts string, err error) {
	const defaultTimestamp = "0001-01-01 00:00:00.0000"
	re := regexp.MustCompile("\"now\"[ ]*[:][ ]*\"[0-9]+\"")
	texts := re.FindAllString(string(extrinsics), 1)
	if len(texts) == 0 {
		return defaultTimestamp, fmt.Errorf("cannot find \"now\" in extrinsics: %w", err)
	}
	stexts := strings.Split(texts[0], "\"")
	if len(stexts) != 5 {
		return defaultTimestamp, fmt.Errorf("cannot find timestamp in extrinsics: len is %d", len(stexts))
	}
	millis, err := strconv.ParseInt(stexts[3], 10, 64)
	if err != nil {
		return defaultTimestamp, fmt.Errorf("cannot convert timestamp to milliseconds: %w", err)
	}
	ts = time.UnixMilli(millis).Format("2006-01-02 15:04:05.0000")
	return
}

// extractAddressesFromExtrinsics extracts Polkadot addresses from extrinsics JSON
func extractAddressesFromExtrinsics(extrinsics json.RawMessage) ([]string, error) {
	if len(extrinsics) == 0 {
		return nil, nil
	}

	// Parse the extrinsics JSON
	var data interface{}
	if err := json.Unmarshal(extrinsics, &data); err != nil {
		return nil, fmt.Errorf("error parsing extrinsics JSON: %w", err)
	}

	// Set to store unique addresses
	addressMap := make(map[string]struct{})

	// Function to validate if a string is a Polkadot address
	validateAddress := func(addr string) bool {
		// Polkadot addresses are 47 or 48 characters long and start with a number or letter
		if len(addr) < 45 || len(addr) > 50 {
			return false
		}

		// Check for common prefixes of Polkadot addresses
		validPrefixes := []string{"1", "5F", "5G", "5D", "5E", "5H"}
		for _, prefix := range validPrefixes {
			if strings.HasPrefix(addr, prefix) {
				return true
			}
		}

		return false
	}

	// Recursive function to find address fields in the JSON
	var findAddresses func(data interface{})
	findAddresses = func(data interface{}) {
		switch v := data.(type) {
		case map[string]interface{}:
			// Check for fields that might contain an address
			for key, value := range v {
				// Check if the field contains "id" in its name and is a string
				if strings.Contains(strings.ToLower(key), "id") {
					if id, ok := value.(string); ok && validateAddress(id) {
						addressMap[id] = struct{}{}
					}
				}
				// Also recursively check all fields
				findAddresses(value)
			}

		case []interface{}:
			// Check each item in the array
			for _, item := range v {
				// If the item is a string, check if it's an address
				if str, ok := item.(string); ok && validateAddress(str) {
					addressMap[str] = struct{}{}
				} else {
					// Otherwise, recursively check the item
					findAddresses(item)
				}
			}
		}
	}

	// Start the recursive search
	findAddresses(data)

	// Convert the map to a slice
	addresses := make([]string, 0, len(addressMap))
	for addr := range addressMap {
		addresses = append(addresses, addr)
	}

	return addresses, nil
}
