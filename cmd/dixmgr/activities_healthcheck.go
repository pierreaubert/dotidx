package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// HTTPHealthCheckConfig configures HTTP health check behavior
type HTTPHealthCheckConfig struct {
	URL              string            // Endpoint URL to check
	Method           string            // HTTP method (GET, POST, etc.)
	Headers          map[string]string // Custom headers
	ExpectedStatus   int               // Expected HTTP status code (0 = any 2xx)
	Timeout          time.Duration     // Request timeout
	ResponseContains string            // Optional: check if response contains this string
	JSONPath         string            // Optional: JSON path to check (e.g., "status.healthy")
}

// HTTPHealthCheckResult contains the result of an HTTP health check
type HTTPHealthCheckResult struct {
	Healthy        bool
	StatusCode     int
	ResponseTime   time.Duration
	ResponseBody   string // Limited to first 1KB
	Error          string
	Timestamp      time.Time
}

// CheckHTTPEndpointActivity performs HTTP health check on an endpoint
// Supports various health check patterns: status code, response content, JSON parsing
func (a *Activities) CheckHTTPEndpointActivity(ctx context.Context, config HTTPHealthCheckConfig) (*HTTPHealthCheckResult, error) {
	start := time.Now()
	result := &HTTPHealthCheckResult{
		Timestamp: start,
		Healthy:   false,
	}

	// Set defaults
	if config.Method == "" {
		config.Method = "GET"
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}
	if config.ExpectedStatus == 0 {
		config.ExpectedStatus = 200
	}

	log.Printf("[Activity] HTTP health check: %s %s", config.Method, config.URL)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: config.Timeout,
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, config.Method, config.URL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create request: %v", err)
		return result, nil // Return result with error, don't fail activity
	}

	// Add custom headers
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("HTTP request failed: %v", err)
		result.ResponseTime = time.Since(start)
		log.Printf("[Activity] HTTP health check failed for %s: %v", config.URL, err)
		return result, nil
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.ResponseTime = time.Since(start)

	// Read response body (limit to 1KB)
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		result.Error = fmt.Sprintf("failed to read response: %v", err)
		return result, nil
	}
	result.ResponseBody = string(bodyBytes)

	// Check status code
	if config.ExpectedStatus > 0 {
		if resp.StatusCode != config.ExpectedStatus {
			result.Error = fmt.Sprintf("unexpected status code: got %d, want %d", resp.StatusCode, config.ExpectedStatus)
			log.Printf("[Activity] HTTP health check unhealthy: %s returned %d", config.URL, resp.StatusCode)
			return result, nil
		}
	} else {
		// Accept any 2xx status code
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			result.Error = fmt.Sprintf("non-2xx status code: %d", resp.StatusCode)
			log.Printf("[Activity] HTTP health check unhealthy: %s returned %d", config.URL, resp.StatusCode)
			return result, nil
		}
	}

	// Check response content if specified
	if config.ResponseContains != "" {
		if !strings.Contains(result.ResponseBody, config.ResponseContains) {
			result.Error = fmt.Sprintf("response does not contain expected string: %s", config.ResponseContains)
			log.Printf("[Activity] HTTP health check unhealthy: response missing expected content")
			return result, nil
		}
	}

	// Check JSON path if specified
	if config.JSONPath != "" {
		healthy, err := checkJSONPath(result.ResponseBody, config.JSONPath)
		if err != nil {
			result.Error = fmt.Sprintf("JSON path check failed: %v", err)
			log.Printf("[Activity] HTTP health check unhealthy: JSON path error: %v", err)
			return result, nil
		}
		if !healthy {
			result.Error = fmt.Sprintf("JSON path %s indicates unhealthy", config.JSONPath)
			log.Printf("[Activity] HTTP health check unhealthy: JSON path check failed")
			return result, nil
		}
	}

	// All checks passed
	result.Healthy = true
	log.Printf("[Activity] HTTP health check passed for %s (status=%d, time=%v)",
		config.URL, result.StatusCode, result.ResponseTime)

	return result, nil
}

// checkJSONPath checks if a JSON path evaluates to true/healthy
// Supports simple paths like "status", "health.ready", etc.
func checkJSONPath(jsonBody, path string) (bool, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonBody), &data); err != nil {
		return false, fmt.Errorf("invalid JSON: %w", err)
	}

	// Split path and traverse
	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			var ok bool
			current, ok = v[part]
			if !ok {
				return false, fmt.Errorf("path not found: %s", part)
			}
		default:
			return false, fmt.Errorf("cannot traverse non-object at: %s", part)
		}
	}

	// Check if final value indicates health
	switch v := current.(type) {
	case bool:
		return v, nil
	case string:
		// Common health string values
		healthy := v == "healthy" || v == "ok" || v == "up" || v == "ready" || v == "true"
		return healthy, nil
	case float64:
		return v > 0, nil
	default:
		return false, fmt.Errorf("unexpected value type at path end: %T", v)
	}
}

// CheckHTTPEndpointSimpleActivity is a simplified version for basic health checks
// Returns true if endpoint responds with 200 OK
func (a *Activities) CheckHTTPEndpointSimpleActivity(ctx context.Context, url string) (bool, error) {
	config := HTTPHealthCheckConfig{
		URL:            url,
		Method:         "GET",
		ExpectedStatus: 200,
		Timeout:        5 * time.Second,
	}

	result, err := a.CheckHTTPEndpointActivity(ctx, config)
	if err != nil {
		return false, err
	}

	return result.Healthy, nil
}
