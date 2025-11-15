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

	// Sync-aware fields
	ServiceName string // Semantic service name for signal/workflow ID generation
	RPCEndpoint string // RPC endpoint URL (if empty, uses http://localhost:RPCPort)
	RPCPort     int    // RPC port for sync checking
	CheckSync   bool   // Whether to check blockchain sync status before marking ready
	ReadySignal string // Signal name to emit when ready (optional override)
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
		Namespace: "dotidx",
		TaskQueue: "dotidx-watcher",
	}
}

// ParaPlan represents configuration for a parachain and its sidecars
type ParaPlan struct {
	ChainID            string             // Chain ID (e.g., "assethub")
	Node               NodeWorkflowConfig // Parachain node configuration
	SidecarServiceName string             // Base name for sidecar services
	SidecarCount       int                // Number of sidecar instances
}

// RelayPlan represents configuration for a relay chain and its parachains
type RelayPlan struct {
	RelayID    string             // Relay chain ID (e.g., "polkadot")
	Node       NodeWorkflowConfig // Relay chain node configuration
	Parachains []ParaPlan         // Parachains attached to this relay
}

// InfrastructureWorkflowInput represents the complete infrastructure orchestration plan
type InfrastructureWorkflowInput struct {
	RelayPlans         []RelayPlan // All relay chains and their parachains
	NginxService       string      // Nginx service name
	AfterNginxServices []string    // Services to start after nginx (dixlive, dixfe, etc.)
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled   bool   // Enable metrics collection
	Port      int    // Metrics server port (default: 9090)
	Namespace string // Prometheus namespace (default: "dixwatcher")
}

// AlertChannelConfig represents configuration for an alert channel
type AlertChannelConfig struct {
	Type    string            // Type: "log", "webhook", "slack", "email"
	Enabled bool              // Enable this channel
	Config  map[string]string // Channel-specific configuration
}

// AlertConfig holds alerting configuration
type AlertConfig struct {
	Enabled        bool                 // Enable alerting
	Channels       []AlertChannelConfig // Alert channels
	DedupeWindow   time.Duration        // Deduplication window (default: 5m)
	EnabledRules   []string             // List of enabled rule names (empty = all)
	DisabledRules  []string             // List of disabled rule names
}

// WatcherConfig represents the complete watcher configuration
type WatcherConfig struct {
	Metrics MetricsConfig // Metrics configuration
	Alerts  AlertConfig   // Alert configuration
	EnableResourceMonitoring bool // Enable resource monitoring for all services
}
