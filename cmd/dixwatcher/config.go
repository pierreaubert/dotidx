package main

import (
	"time"
)

// NodeWorkflowConfig represents configuration for a single service node workflow
type NodeWorkflowConfig struct {
	Name             string        // Logical name of the service
	SystemdUnit      string        // Systemd unit name (e.g., "nginx.service")
	WatchInterval    time.Duration // How often to check service health
	MaxRestarts      int           // Maximum restart attempts before giving up
	RestartBackoff   time.Duration // Base backoff duration between restart attempts
	ParentWorkflowID string        // ID of parent workflow for signaling
}

// ClusterWorkflowConfig represents configuration for managing redundant services
type ClusterWorkflowConfig struct {
	Name         string               // Cluster name (e.g., "RelayChainCluster")
	ReplicaCount int                  // Number of replicas to run
	Quorum       int                  // Minimum healthy replicas required
	Nodes        []NodeWorkflowConfig // Configuration for each node
}

// DependencyInfo represents a dependency relationship
type DependencyInfo struct {
	WorkflowID   string   // Workflow ID to wait for
	SignalNames  []string // Signal names that indicate readiness
	RequiredAny  bool     // If true, wait for any signal; if false, wait for all
	TimeoutHours int      // How long to wait for dependency before failing
}

// DependentServiceConfig represents a service that depends on other services
type DependentServiceConfig struct {
	NodeConfig   NodeWorkflowConfig // Base service configuration
	Dependencies []DependencyInfo   // List of dependencies to wait for
}

// TemporalConfig holds Temporal connection settings
type TemporalConfig struct {
	HostPort  string // Temporal server address (e.g., "localhost:7233")
	Namespace string // Temporal namespace (e.g., "dotidx")
	TaskQueue string // Task queue name (e.g., "dotidx-watcher")
}

// GetDefaultTemporalConfig returns default Temporal configuration
func GetDefaultTemporalConfig() TemporalConfig {
	return TemporalConfig{
		HostPort:  "localhost:7233",
		Namespace: "default",
		TaskQueue: "dotidx-watcher",
	}
}
