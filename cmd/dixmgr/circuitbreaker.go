package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker
type CircuitState string

const (
	StateClosed    CircuitState = "closed"    // Normal operation, requests pass through
	StateOpen      CircuitState = "open"      // Failing, requests are rejected
	StateHalfOpen  CircuitState = "half_open" // Testing if service recovered
)

// CircuitBreaker implements the circuit breaker pattern
// Prevents cascading failures by failing fast when a service is degraded
type CircuitBreaker struct {
	name              string
	maxFailures       int           // Number of failures before opening
	timeout           time.Duration // How long to wait before trying half-open
	halfOpenRequests  int           // Number of test requests in half-open state

	state             CircuitState
	failures          int
	successes         int
	lastFailureTime   time.Time
	lastStateChange   time.Time
	consecutiveSuccess int

	mu                sync.RWMutex
	metrics           *MetricsCollector
}

// CircuitBreakerConfig configures a circuit breaker
type CircuitBreakerConfig struct {
	Name             string        // Circuit name (usually service name)
	MaxFailures      int           // Failures before opening (default: 5)
	Timeout          time.Duration // Time before half-open retry (default: 60s)
	HalfOpenRequests int           // Test requests in half-open (default: 3)
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig, metrics *MetricsCollector) *CircuitBreaker {
	// Set defaults
	if config.MaxFailures == 0 {
		config.MaxFailures = 5
	}
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}
	if config.HalfOpenRequests == 0 {
		config.HalfOpenRequests = 3
	}

	cb := &CircuitBreaker{
		name:             config.Name,
		maxFailures:      config.MaxFailures,
		timeout:          config.Timeout,
		halfOpenRequests: config.HalfOpenRequests,
		state:            StateClosed,
		lastStateChange:  time.Now(),
		metrics:          metrics,
	}

	return cb
}

// Call executes a function with circuit breaker protection
func (cb *CircuitBreaker) Call(ctx context.Context, fn func() error) error {
	cb.mu.Lock()

	// Check if we should transition from open to half-open
	if cb.state == StateOpen {
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.setState(StateHalfOpen)
			cb.consecutiveSuccess = 0
		} else {
			cb.mu.Unlock()
			return fmt.Errorf("circuit breaker is open for %s", cb.name)
		}
	}

	// Reject requests if open
	if cb.state == StateOpen {
		cb.mu.Unlock()
		return fmt.Errorf("circuit breaker is open for %s", cb.name)
	}

	cb.mu.Unlock()

	// Execute the function
	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}

	return err
}

// recordFailure records a failure and potentially opens the circuit
func (cb *CircuitBreaker) recordFailure() {
	cb.failures++
	cb.lastFailureTime = time.Now()
	cb.consecutiveSuccess = 0

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.maxFailures {
			cb.setState(StateOpen)
		}
	case StateHalfOpen:
		// Any failure in half-open immediately reopens the circuit
		cb.setState(StateOpen)
	}

	// Record metrics
	if cb.metrics != nil {
		cb.metrics.RecordActivityError("CircuitBreaker", "failure")
	}
}

// recordSuccess records a success and potentially closes the circuit
func (cb *CircuitBreaker) recordSuccess() {
	cb.successes++
	cb.consecutiveSuccess++

	switch cb.state {
	case StateHalfOpen:
		if cb.consecutiveSuccess >= cb.halfOpenRequests {
			// Enough successes in half-open, close the circuit
			cb.setState(StateClosed)
			cb.failures = 0
		}
	case StateClosed:
		// Reset failure count on success
		if cb.consecutiveSuccess >= cb.maxFailures {
			cb.failures = 0
		}
	}
}

// setState changes the circuit breaker state
func (cb *CircuitBreaker) setState(newState CircuitState) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState
	cb.lastStateChange = time.Now()

	// Log state transition
	transitionMsg := fmt.Sprintf("Circuit breaker %s: %s -> %s (failures: %d, successes: %d)",
		cb.name, oldState, newState, cb.failures, cb.successes)

	// Use appropriate log level based on state
	switch newState {
	case StateOpen:
		// Circuit opened - this is critical
		if cb.metrics != nil {
			cb.metrics.RecordActivityExecution("CircuitBreaker", "opened")
		}
		fmt.Printf("[CIRCUIT BREAKER] CRITICAL: %s\n", transitionMsg)
	case StateHalfOpen:
		// Testing recovery
		fmt.Printf("[CIRCUIT BREAKER] INFO: %s\n", transitionMsg)
	case StateClosed:
		// Recovered
		if cb.metrics != nil {
			cb.metrics.RecordActivityExecution("CircuitBreaker", "closed")
		}
		fmt.Printf("[CIRCUIT BREAKER] INFO: %s\n", transitionMsg)
	}
}

// GetState returns the current state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns current statistics
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		Name:              cb.name,
		State:             cb.state,
		Failures:          cb.failures,
		Successes:         cb.successes,
		ConsecutiveSuccess: cb.consecutiveSuccess,
		LastFailureTime:   cb.lastFailureTime,
		LastStateChange:   cb.lastStateChange,
	}
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.setState(StateClosed)
	cb.failures = 0
	cb.successes = 0
	cb.consecutiveSuccess = 0
}

// CircuitBreakerStats holds circuit breaker statistics
type CircuitBreakerStats struct {
	Name              string
	State             CircuitState
	Failures          int
	Successes         int
	ConsecutiveSuccess int
	LastFailureTime   time.Time
	LastStateChange   time.Time
}

// CircuitBreakerManager manages multiple circuit breakers
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
	metrics  *MetricsCollector
	config   CircuitBreakerConfig // Default config
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager(defaultConfig CircuitBreakerConfig, metrics *MetricsCollector) *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
		metrics:  metrics,
		config:   defaultConfig,
	}
}

// GetOrCreate gets an existing circuit breaker or creates a new one
func (m *CircuitBreakerManager) GetOrCreate(name string) *CircuitBreaker {
	m.mu.RLock()
	cb, exists := m.breakers[name]
	m.mu.RUnlock()

	if exists {
		return cb
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, exists := m.breakers[name]; exists {
		return cb
	}

	// Create new circuit breaker
	config := m.config
	config.Name = name
	cb = NewCircuitBreaker(config, m.metrics)
	m.breakers[name] = cb

	return cb
}

// Get returns an existing circuit breaker or nil
func (m *CircuitBreakerManager) Get(name string) *CircuitBreaker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.breakers[name]
}

// GetAllStats returns statistics for all circuit breakers
func (m *CircuitBreakerManager) GetAllStats() []CircuitBreakerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make([]CircuitBreakerStats, 0, len(m.breakers))
	for _, cb := range m.breakers {
		stats = append(stats, cb.GetStats())
	}
	return stats
}

// ResetAll resets all circuit breakers
func (m *CircuitBreakerManager) ResetAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, cb := range m.breakers {
		cb.Reset()
	}
}

// Count returns the number of circuit breakers
func (m *CircuitBreakerManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.breakers)
}
