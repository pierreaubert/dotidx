package main

import (
	"context"
	"time"
)

// ProcessManager defines the interface for managing processes
// Supports multiple backends: systemd, direct, kubernetes, etc.
type ProcessManager interface {
	// Start starts a process
	Start(ctx context.Context, config ProcessConfig) error

	// Stop stops a process
	Stop(ctx context.Context, name string) error

	// Restart restarts a process
	Restart(ctx context.Context, name string) error

	// GetStatus returns the current status of a process
	GetStatus(ctx context.Context, name string) (*ProcessStatus, error)

	// GetOutput returns recent output from a process
	GetOutput(ctx context.Context, name string, lines int) ([]string, error)

	// Kill forcefully kills a process
	Kill(ctx context.Context, name string) error

	// List returns all managed processes
	List(ctx context.Context) ([]string, error)

	// Close cleans up resources
	Close() error

	// Name returns the manager type
	Name() string
}

// ProcessConfig defines how to start a process
type ProcessConfig struct {
	// Basic configuration
	Name        string   // Unique process name
	Command     string   // Command to execute
	Args        []string // Command arguments
	WorkingDir  string   // Working directory
	Environment []string // Environment variables (KEY=VALUE format)

	// User/Group (requires root or capabilities)
	User  string // Username or UID
	Group string // Group name or GID

	// Resource limits
	CPULimit    float64 // CPU limit in cores (0 = unlimited)
	MemoryLimit int64   // Memory limit in bytes (0 = unlimited)
	IOWeight    int     // I/O weight (0-1000, 0 = default)

	// Restart policy
	RestartPolicy RestartPolicy
	RestartDelay  time.Duration

	// Output handling
	CaptureOutput bool   // Whether to capture stdout/stderr
	LogFile       string // Optional log file path

	// Health checking
	HealthCheck *HealthCheckConfig
}

// RestartPolicy defines when to restart a process
type RestartPolicy string

const (
	RestartNever     RestartPolicy = "never"      // Never restart
	RestartOnFailure RestartPolicy = "on-failure" // Restart only on failure
	RestartAlways    RestartPolicy = "always"     // Always restart
)

// HealthCheckConfig defines health check parameters
type HealthCheckConfig struct {
	Type     string        // "http", "tcp", "exec"
	Endpoint string        // For HTTP/TCP
	Command  string        // For exec
	Interval time.Duration // Check interval
	Timeout  time.Duration // Check timeout
	Retries  int           // Failures before unhealthy
}

// ProcessStatus represents the current status of a process
type ProcessStatus struct {
	Name        string
	State       ProcessState
	PID         int
	StartTime   time.Time
	RestartCount int
	ExitCode    int
	Error       string
	CPUPercent  float64
	MemoryBytes int64
	Healthy     bool
}

// ProcessState represents the state of a process
type ProcessState string

const (
	StateUnknown  ProcessState = "unknown"
	StateStopped  ProcessState = "stopped"
	StateStarting ProcessState = "starting"
	StateRunning  ProcessState = "running"
	StateStopping ProcessState = "stopping"
	StateFailed   ProcessState = "failed"
)

// ProcessManagerType defines the type of process manager
type ProcessManagerType string

const (
	ProcessManagerSystemd ProcessManagerType = "systemd"
	ProcessManagerDirect  ProcessManagerType = "direct"
)

// ProcessManagerConfig configures the process manager
type ProcessManagerConfig struct {
	Type ProcessManagerType // Which manager to use

	// Systemd-specific
	SystemdNamespace string // Systemd namespace (optional)

	// Direct-specific
	LogDir           string // Directory for process logs
	PIDDir           string // Directory for PID files
	MaxRestarts      int    // Maximum restart attempts
	UseCgroups       bool   // Whether to use cgroups for resource limits
}

// NewProcessManager creates a new process manager based on configuration
func NewProcessManager(config ProcessManagerConfig, metrics *MetricsCollector) (ProcessManager, error) {
	switch config.Type {
	case ProcessManagerSystemd:
		return NewSystemdManager(config, metrics)
	case ProcessManagerDirect:
		return NewDirectManager(config, metrics)
	default:
		return NewSystemdManager(config, metrics) // Default to systemd
	}
}
