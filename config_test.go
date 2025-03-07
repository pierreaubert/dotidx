package main

import (
	"flag"
	"testing"
	"time"
)

func TestParseFlags(t *testing.T) {
	// Create a temporary function that mimics parseFlags but uses a new FlagSet
	parseTestFlags := func(args []string) Config {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		startRange := fs.Int("start", 1, "Start of the integer range")
		endRange := fs.Int("end", 10, "End of the integer range")
		sidecarURL := fs.String("sidecar", "", "Sidecar URL")
		postgresURI := fs.String("postgres", "", "PostgreSQL connection URI")
		batchSize := fs.Int("batch", 100, "Number of items to collect before writing to database")
		maxWorkers := fs.Int("workers", 5, "Maximum number of concurrent workers")
		flushTimeout := fs.Duration("flush", 30*time.Second, "Maximum time to wait before flushing data to database")

		// Parse the provided args (skip the program name)
		fs.Parse(args[1:])

		return Config{
			StartRange:   *startRange,
			EndRange:     *endRange,
			SidecarURL:   *sidecarURL,
			PostgresURI:  *postgresURI,
			BatchSize:    *batchSize,
			MaxWorkers:   *maxWorkers,
			FlushTimeout: *flushTimeout,
		}
	}

	tests := []struct {
		name     string
		args     []string
		expected Config
	}{
		{
			name: "Default values",
			args: []string{"cmd"},
			expected: Config{
				StartRange:   1,
				EndRange:     10,
				SidecarURL:   "",
				PostgresURI:  "",
				BatchSize:    100,
				MaxWorkers:   5,
				FlushTimeout: 30 * time.Second,
			},
		},
		{
			name: "Custom values",
			args: []string{
				"cmd",
				"-start=5",
				"-end=15",
				"-sidecar=http://example.com/sidecar",
				"-postgres=postgres://user:pass@localhost:5432/db",
				"-batch=50",
				"-workers=10",
				"-flush=1m",
			},
			expected: Config{
				StartRange:   5,
				EndRange:     15,
				SidecarURL:   "http://example.com/sidecar",
				PostgresURI:  "postgres://user:pass@localhost:5432/db",
				BatchSize:    50,
				MaxWorkers:   10,
				FlushTimeout: 1 * time.Minute,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := parseTestFlags(tc.args)

			if config.StartRange != tc.expected.StartRange {
				t.Errorf("Expected StartRange=%d, got %d", tc.expected.StartRange, config.StartRange)
			}
			if config.EndRange != tc.expected.EndRange {
				t.Errorf("Expected EndRange=%d, got %d", tc.expected.EndRange, config.EndRange)
			}
			if config.SidecarURL != tc.expected.SidecarURL {
				t.Errorf("Expected SidecarURL=%s, got %s", tc.expected.SidecarURL, config.SidecarURL)
			}
			if config.PostgresURI != tc.expected.PostgresURI {
				t.Errorf("Expected PostgresURI=%s, got %s", tc.expected.PostgresURI, config.PostgresURI)
			}
			if config.BatchSize != tc.expected.BatchSize {
				t.Errorf("Expected BatchSize=%d, got %d", tc.expected.BatchSize, config.BatchSize)
			}
			if config.MaxWorkers != tc.expected.MaxWorkers {
				t.Errorf("Expected MaxWorkers=%d, got %d", tc.expected.MaxWorkers, config.MaxWorkers)
			}
			if config.FlushTimeout != tc.expected.FlushTimeout {
				t.Errorf("Expected FlushTimeout=%v, got %v", tc.expected.FlushTimeout, config.FlushTimeout)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name: "Valid config",
			config: Config{
				StartRange:   1,
				EndRange:     10,
				SidecarURL:   "http://example.com/sidecar",
				PostgresURI:  "postgres://user:pass@localhost:5432/db",
				BatchSize:    100,
				MaxWorkers:   5,
				FlushTimeout: 30 * time.Second,
			},
			expectError: false,
		},
		{
			name: "Start range greater than end range",
			config: Config{
				StartRange:   10,
				EndRange:     1,
				SidecarURL:   "http://example.com/sidecar",
				PostgresURI:  "postgres://user:pass@localhost:5432/db",
				BatchSize:    100,
				MaxWorkers:   5,
				FlushTimeout: 30 * time.Second,
			},
			expectError: true,
		},
		{
			name: "Missing Sidecar URL",
			config: Config{
				StartRange:   1,
				EndRange:     10,
				SidecarURL:   "",
				PostgresURI:  "postgres://user:pass@localhost:5432/db",
				BatchSize:    100,
				MaxWorkers:   5,
				FlushTimeout: 30 * time.Second,
			},
			expectError: true,
		},
		{
			name: "Missing PostgreSQL URI",
			config: Config{
				StartRange:   1,
				EndRange:     10,
				SidecarURL:   "http://example.com/sidecar",
				PostgresURI:  "",
				BatchSize:    100,
				MaxWorkers:   5,
				FlushTimeout: 30 * time.Second,
			},
			expectError: true,
		},
		{
			name: "Invalid batch size",
			config: Config{
				StartRange:   1,
				EndRange:     10,
				SidecarURL:   "http://example.com/sidecar",
				PostgresURI:  "postgres://user:pass@localhost:5432/db",
				BatchSize:    0,
				MaxWorkers:   5,
				FlushTimeout: 30 * time.Second,
			},
			expectError: true,
		},
		{
			name: "Invalid max workers",
			config: Config{
				StartRange:   1,
				EndRange:     10,
				SidecarURL:   "http://example.com/sidecar",
				PostgresURI:  "postgres://user:pass@localhost:5432/db",
				BatchSize:    100,
				MaxWorkers:   0,
				FlushTimeout: 30 * time.Second,
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfig(tc.config)
			if (err != nil) != tc.expectError {
				t.Errorf("Expected error=%v, got error=%v", tc.expectError, err != nil)
			}
		})
	}
}
