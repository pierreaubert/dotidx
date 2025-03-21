package dotidx

import (
	"fmt"
	"strconv"
	"time"
)

// ParseTimestamp parses a timestamp string in different formats and returns a time.Time.
// It tries multiple formats:
// - Unix timestamp (seconds since epoch)
// - RFC3339 format
// - ISO8601 format
func ParseTimestamp(timestamp string) (time.Time, error) {
	// Try parsing as Unix timestamp (seconds since epoch)
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err == nil {
		return time.Unix(seconds, 0), nil
	}

	// Try RFC3339 format
	t, err := time.Parse(time.RFC3339, timestamp)
	if err == nil {
		return t, nil
	}

	// Try ISO8601 format
	formats := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		t, err := time.Parse(format, timestamp)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", timestamp)
}
