package dotidx

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestEventsBalanceProcess(t *testing.T) {
	// Test cases for the EventsBalance filter
	testCases := []struct {
		name     string
		address  string
		Input    json.RawMessage
		Expected json.RawMessage
		IsError  bool
	}{
		{
			name:    "Multiple matching events",
			address: "12e1d9wD5hpQuE7EMP8h78giqB8z7pU8pUrw8RGxuVtozNRZ",
			Input: func() json.RawMessage {
				data, err := os.ReadFile("./tests/data/blocks/ex-24731329.json")
				if err != nil {
					t.Fatalf("Failed to read test data: %v", err)
				}
				return json.RawMessage(data)
			}(),
			Expected: json.RawMessage(`
[
 {
    "method": {
      "pallet": "balances",
      "method": "Withdraw"
    },
    "data": ["12e1d9wD5hpQuE7EMP8h78giqB8z7pU8pUrw8RGxuVtozNRZ", "159154905"]
  },
  {
    "method": {
      "pallet": "balances",
      "method": "Transfer"
    },
    "data": [
      "12e1d9wD5hpQuE7EMP8h78giqB8z7pU8pUrw8RGxuVtozNRZ",
      "16ffFuCAuwuSAbrrmsfTUGaqVt6vyJBcsrk82duBMqdcr6nW",
      "745044000000"
    ]
  },
  {
    "method": {
      "pallet": "balances",
      "method": "Deposit"
    },
    "data": ["12e1d9wD5hpQuE7EMP8h78giqB8z7pU8pUrw8RGxuVtozNRZ", "0"]
  },
  {
    "method": {
      "pallet": "transactionPayment",
      "method": "TransactionFeePaid"
    },
    "data": [
      "12e1d9wD5hpQuE7EMP8h78giqB8z7pU8pUrw8RGxuVtozNRZ",
      "159154905",
      "0"
    ]
  }
]`),
			IsError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new filter
			filter := NewEventsBalance(tc.address)

			// Process the input
			filtered, err := filter.Process(tc.Input)

			// Check error condition
			if tc.IsError && err == nil {
				t.Errorf("Expected an error but got none")
			}
			if !tc.IsError && err != nil {
				t.Errorf("Did not expect an error but got: %v", err)
			}

			// If we expect no error, check the output
			if !tc.IsError {
				// Compare the actual output with the expected output
				var actual, expected interface{}
				if err := json.Unmarshal(filtered, &actual); err != nil {
					t.Fatalf("Failed to unmarshal actual result: %v", err)
				}
				if err := json.Unmarshal(tc.Expected, &expected); err != nil {
					t.Fatalf("Failed to unmarshal expected result: %v", err)
				}

				if !reflect.DeepEqual(actual, expected) {
					actualJSON, _ := json.MarshalIndent(actual, "", "  ")
					expectedJSON, _ := json.MarshalIndent(expected, "", "  ")
					t.Errorf("Results don't match\nActual: %s\nExpected: %s", actualJSON, expectedJSON)
				}
			}
		})
	}
}

func TestFilterInterface(t *testing.T) {
	// This test ensures that EventsBalance correctly implements the Filter interface
	var _ Filter = (*EventsBalance)(nil)
}

func TestNewEventsBalance(t *testing.T) {
	// Test the constructor
	address := "13UVJyLnbVp9RBZYFwFGyDvVd1y27Tt8tkntv6Q7JVPhFsTB"
	filter := NewEventsBalance(address)

	// Check that the filter was created with the correct address
	if filter.address != address {
		t.Errorf("Expected address to be %s, got %s", address, filter.address)
	}
}
