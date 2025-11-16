package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// DynamicConfig manages runtime configuration updates
type DynamicConfig struct {
	mu sync.RWMutex

	// Feature flags
	MetricsEnabled           bool
	AlertsEnabled            bool
	ResourceMonitoringEnabled bool
	HealthHistoryEnabled     bool

	// Thresholds (can be updated at runtime)
	CPUWarningThreshold      float64
	CPUCriticalThreshold     float64
	MemoryWarningThreshold   int64
	MemoryCriticalThreshold  int64
	DiskIOWarningThreshold   float64

	// Circuit breaker settings
	CircuitBreakerMaxFailures int
	CircuitBreakerTimeout     time.Duration

	// Alert settings
	AlertDedupeWindow time.Duration

	// Metrics settings
	MetricsPort      int
	MetricsNamespace string

	// Health history settings
	HealthHistoryDBPath        string
	HealthHistoryRetentionDays int

	// Callbacks for config changes
	onChange []func(old, new *DynamicConfig)
}

// NewDynamicConfig creates a new dynamic configuration
func NewDynamicConfig() *DynamicConfig {
	return &DynamicConfig{
		// Defaults
		MetricsEnabled:            true,
		AlertsEnabled:             true,
		ResourceMonitoringEnabled: true,
		HealthHistoryEnabled:      false,

		CPUWarningThreshold:     80.0,
		CPUCriticalThreshold:    95.0,
		MemoryWarningThreshold:  2 * 1024 * 1024 * 1024, // 2GB
		MemoryCriticalThreshold: 4 * 1024 * 1024 * 1024, // 4GB
		DiskIOWarningThreshold:  100 * 1024 * 1024,      // 100 MB/s

		CircuitBreakerMaxFailures: 5,
		CircuitBreakerTimeout:     60 * time.Second,

		AlertDedupeWindow: 5 * time.Minute,

		MetricsPort:      9090,
		MetricsNamespace: "dixmgr",

		HealthHistoryDBPath:        "/var/lib/dixmgr/health.db",
		HealthHistoryRetentionDays: 30,

		onChange: make([]func(old, new *DynamicConfig), 0),
	}
}

// Clone creates a deep copy of the configuration
func (c *DynamicConfig) Clone() *DynamicConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	clone := &DynamicConfig{
		MetricsEnabled:            c.MetricsEnabled,
		AlertsEnabled:             c.AlertsEnabled,
		ResourceMonitoringEnabled: c.ResourceMonitoringEnabled,
		HealthHistoryEnabled:      c.HealthHistoryEnabled,

		CPUWarningThreshold:     c.CPUWarningThreshold,
		CPUCriticalThreshold:    c.CPUCriticalThreshold,
		MemoryWarningThreshold:  c.MemoryWarningThreshold,
		MemoryCriticalThreshold: c.MemoryCriticalThreshold,
		DiskIOWarningThreshold:  c.DiskIOWarningThreshold,

		CircuitBreakerMaxFailures: c.CircuitBreakerMaxFailures,
		CircuitBreakerTimeout:     c.CircuitBreakerTimeout,

		AlertDedupeWindow: c.AlertDedupeWindow,

		MetricsPort:      c.MetricsPort,
		MetricsNamespace: c.MetricsNamespace,

		HealthHistoryDBPath:        c.HealthHistoryDBPath,
		HealthHistoryRetentionDays: c.HealthHistoryRetentionDays,

		onChange: c.onChange,
	}

	return clone
}

// Update updates the configuration
func (c *DynamicConfig) Update(updates map[string]interface{}) error {
	c.mu.Lock()
	oldConfig := c.Clone()
	c.mu.Unlock()

	// Apply updates
	for key, value := range updates {
		if err := c.set(key, value); err != nil {
			return fmt.Errorf("failed to set %s: %w", key, err)
		}
	}

	// Notify listeners
	c.mu.RLock()
	callbacks := c.onChange
	c.mu.RUnlock()

	for _, callback := range callbacks {
		callback(oldConfig, c)
	}

	log.Printf("Configuration updated: %d fields changed", len(updates))
	return nil
}

// set sets a single configuration value
func (c *DynamicConfig) set(key string, value interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch key {
	case "metrics_enabled":
		c.MetricsEnabled = value.(bool)
	case "alerts_enabled":
		c.AlertsEnabled = value.(bool)
	case "resource_monitoring_enabled":
		c.ResourceMonitoringEnabled = value.(bool)
	case "health_history_enabled":
		c.HealthHistoryEnabled = value.(bool)

	case "cpu_warning_threshold":
		c.CPUWarningThreshold = value.(float64)
	case "cpu_critical_threshold":
		c.CPUCriticalThreshold = value.(float64)
	case "memory_warning_threshold":
		c.MemoryWarningThreshold = value.(int64)
	case "memory_critical_threshold":
		c.MemoryCriticalThreshold = value.(int64)
	case "disk_io_warning_threshold":
		c.DiskIOWarningThreshold = value.(float64)

	case "circuit_breaker_max_failures":
		c.CircuitBreakerMaxFailures = value.(int)
	case "circuit_breaker_timeout":
		c.CircuitBreakerTimeout = value.(time.Duration)

	case "alert_dedupe_window":
		c.AlertDedupeWindow = value.(time.Duration)

	case "metrics_port":
		c.MetricsPort = value.(int)
	case "metrics_namespace":
		c.MetricsNamespace = value.(string)

	case "health_history_db_path":
		c.HealthHistoryDBPath = value.(string)
	case "health_history_retention_days":
		c.HealthHistoryRetentionDays = value.(int)

	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}

	return nil
}

// Get returns a configuration value
func (c *DynamicConfig) Get(key string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	switch key {
	case "metrics_enabled":
		return c.MetricsEnabled, nil
	case "alerts_enabled":
		return c.AlertsEnabled, nil
	case "resource_monitoring_enabled":
		return c.ResourceMonitoringEnabled, nil
	case "health_history_enabled":
		return c.HealthHistoryEnabled, nil

	case "cpu_warning_threshold":
		return c.CPUWarningThreshold, nil
	case "cpu_critical_threshold":
		return c.CPUCriticalThreshold, nil
	case "memory_warning_threshold":
		return c.MemoryWarningThreshold, nil
	case "memory_critical_threshold":
		return c.MemoryCriticalThreshold, nil
	case "disk_io_warning_threshold":
		return c.DiskIOWarningThreshold, nil

	case "circuit_breaker_max_failures":
		return c.CircuitBreakerMaxFailures, nil
	case "circuit_breaker_timeout":
		return c.CircuitBreakerTimeout, nil

	case "alert_dedupe_window":
		return c.AlertDedupeWindow, nil

	case "metrics_port":
		return c.MetricsPort, nil
	case "metrics_namespace":
		return c.MetricsNamespace, nil

	case "health_history_db_path":
		return c.HealthHistoryDBPath, nil
	case "health_history_retention_days":
		return c.HealthHistoryRetentionDays, nil

	default:
		return nil, fmt.Errorf("unknown configuration key: %s", key)
	}
}

// OnChange registers a callback for configuration changes
func (c *DynamicConfig) OnChange(callback func(old, new *DynamicConfig)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onChange = append(c.onChange, callback)
}

// LoadFromFile loads configuration from a TOML file
func (c *DynamicConfig) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse TOML: %w", err)
	}

	return c.Update(config)
}

// SaveToFile saves configuration to a TOML file
func (c *DynamicConfig) SaveToFile(path string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	config := map[string]interface{}{
		"metrics_enabled":             c.MetricsEnabled,
		"alerts_enabled":              c.AlertsEnabled,
		"resource_monitoring_enabled": c.ResourceMonitoringEnabled,
		"health_history_enabled":      c.HealthHistoryEnabled,

		"cpu_warning_threshold":     c.CPUWarningThreshold,
		"cpu_critical_threshold":    c.CPUCriticalThreshold,
		"memory_warning_threshold":  c.MemoryWarningThreshold,
		"memory_critical_threshold": c.MemoryCriticalThreshold,
		"disk_io_warning_threshold": c.DiskIOWarningThreshold,

		"circuit_breaker_max_failures": c.CircuitBreakerMaxFailures,
		"circuit_breaker_timeout":      c.CircuitBreakerTimeout.String(),

		"alert_dedupe_window": c.AlertDedupeWindow.String(),

		"metrics_port":      c.MetricsPort,
		"metrics_namespace": c.MetricsNamespace,

		"health_history_db_path":        c.HealthHistoryDBPath,
		"health_history_retention_days": c.HealthHistoryRetentionDays,
	}

	data, err := toml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal TOML: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ToJSON returns the configuration as JSON
func (c *DynamicConfig) ToJSON() ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	config := map[string]interface{}{
		"metrics_enabled":             c.MetricsEnabled,
		"alerts_enabled":              c.AlertsEnabled,
		"resource_monitoring_enabled": c.ResourceMonitoringEnabled,
		"health_history_enabled":      c.HealthHistoryEnabled,

		"cpu_warning_threshold":     c.CPUWarningThreshold,
		"cpu_critical_threshold":    c.CPUCriticalThreshold,
		"memory_warning_threshold":  c.MemoryWarningThreshold,
		"memory_critical_threshold": c.MemoryCriticalThreshold,
		"disk_io_warning_threshold": c.DiskIOWarningThreshold,

		"circuit_breaker_max_failures": c.CircuitBreakerMaxFailures,
		"circuit_breaker_timeout":      c.CircuitBreakerTimeout.String(),

		"alert_dedupe_window": c.AlertDedupeWindow.String(),

		"metrics_port":      c.MetricsPort,
		"metrics_namespace": c.MetricsNamespace,

		"health_history_db_path":        c.HealthHistoryDBPath,
		"health_history_retention_days": c.HealthHistoryRetentionDays,
	}

	return json.MarshalIndent(config, "", "  ")
}

// ConfigHTTPServer provides HTTP endpoints for dynamic configuration
type ConfigHTTPServer struct {
	config *DynamicConfig
	mu     sync.RWMutex
}

// NewConfigHTTPServer creates a new HTTP server for configuration
func NewConfigHTTPServer(config *DynamicConfig) *ConfigHTTPServer {
	return &ConfigHTTPServer{
		config: config,
	}
}

// HandleGetConfig returns the current configuration
func (s *ConfigHTTPServer) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := s.config.ToJSON()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to serialize config: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// HandleUpdateConfig updates configuration values
func (s *ConfigHTTPServer) HandleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var updates map[string]interface{}
	if err := json.Unmarshal(body, &updates); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if err := s.config.Update(updates); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update config: %v", err), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","message":"Configuration updated"}`))
}

// HandleReloadConfig reloads configuration from file
func (s *ConfigHTTPServer) HandleReloadConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	configPath := r.URL.Query().Get("path")
	if configPath == "" {
		http.Error(w, "Missing 'path' query parameter", http.StatusBadRequest)
		return
	}

	if err := s.config.LoadFromFile(configPath); err != nil {
		http.Error(w, fmt.Sprintf("Failed to reload config: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","message":"Configuration reloaded"}`))
}

// RegisterHandlers registers HTTP handlers for configuration management
func (s *ConfigHTTPServer) RegisterHandlers() {
	http.HandleFunc("/config", s.HandleGetConfig)
	http.HandleFunc("/config/update", s.HandleUpdateConfig)
	http.HandleFunc("/config/reload", s.HandleReloadConfig)
}
