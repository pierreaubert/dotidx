package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// DirectManager manages processes directly without systemd
type DirectManager struct {
	config    ProcessManagerConfig
	metrics   *MetricsCollector
	processes map[string]*ManagedProcess
	mu        sync.RWMutex
	logDir    string
	pidDir    string
}

// ManagedProcess represents a process managed directly
type ManagedProcess struct {
	Config       ProcessConfig
	Cmd          *exec.Cmd
	State        ProcessState
	PID          int
	StartTime    time.Time
	RestartCount int
	ExitCode     int
	Error        string
	Output       *RingBuffer // Ring buffer for recent output
	LogFile      *os.File
	cancel       context.CancelFunc
	mu           sync.RWMutex
}

// RingBuffer stores recent output lines
type RingBuffer struct {
	lines    []string
	size     int
	position int
	mu       sync.RWMutex
}

// NewRingBuffer creates a new ring buffer
func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		lines: make([]string, size),
		size:  size,
	}
}

// Add adds a line to the ring buffer
func (rb *RingBuffer) Add(line string) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.lines[rb.position] = line
	rb.position = (rb.position + 1) % rb.size
}

// GetLines returns the last n lines
func (rb *RingBuffer) GetLines(n int) []string {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n > rb.size {
		n = rb.size
	}

	result := make([]string, 0, n)
	for i := 0; i < n; i++ {
		idx := (rb.position - n + i + rb.size) % rb.size
		if rb.lines[idx] != "" {
			result = append(result, rb.lines[idx])
		}
	}

	return result
}

// NewDirectManager creates a new direct process manager
func NewDirectManager(config ProcessManagerConfig, metrics *MetricsCollector) (*DirectManager, error) {
	logDir := config.LogDir
	if logDir == "" {
		logDir = "/var/log/dixwatcher"
	}

	pidDir := config.PIDDir
	if pidDir == "" {
		pidDir = "/var/run/dixwatcher"
	}

	// Create directories
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create PID directory: %w", err)
	}

	dm := &DirectManager{
		config:    config,
		metrics:   metrics,
		processes: make(map[string]*ManagedProcess),
		logDir:    logDir,
		pidDir:    pidDir,
	}

	return dm, nil
}

// Name returns the manager type
func (m *DirectManager) Name() string {
	return "direct"
}

// Start starts a process directly
func (m *DirectManager) Start(ctx context.Context, config ProcessConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already running
	if proc, exists := m.processes[config.Name]; exists {
		if proc.State == StateRunning || proc.State == StateStarting {
			return fmt.Errorf("process %s is already running", config.Name)
		}
	}

	// Create process context
	procCtx, cancel := context.WithCancel(context.Background())

	// Create command
	cmd := exec.CommandContext(procCtx, config.Command, config.Args...)

	// Set working directory
	if config.WorkingDir != "" {
		cmd.Dir = config.WorkingDir
	}

	// Set environment
	if len(config.Environment) > 0 {
		cmd.Env = append(os.Environ(), config.Environment...)
	}

	// Set user/group if specified (requires root)
	if config.User != "" {
		uid, gid, err := lookupUser(config.User, config.Group)
		if err != nil {
			cancel()
			return fmt.Errorf("failed to lookup user: %w", err)
		}

		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: uid,
				Gid: gid,
			},
		}
	}

	// Create managed process
	proc := &ManagedProcess{
		Config:    config,
		Cmd:       cmd,
		State:     StateStarting,
		StartTime: time.Now(),
		Output:    NewRingBuffer(1000), // Store last 1000 lines
		cancel:    cancel,
	}

	// Set up output capture
	if config.CaptureOutput {
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			cancel()
			return fmt.Errorf("failed to create stdout pipe: %w", err)
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			cancel()
			return fmt.Errorf("failed to create stderr pipe: %w", err)
		}

		// Open log file if specified
		if config.LogFile != "" {
			logFile, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				log.Printf("Warning: failed to open log file %s: %v", config.LogFile, err)
			} else {
				proc.LogFile = logFile
			}
		}

		// Start output capture goroutines
		go m.captureOutput(proc, stdout, "stdout")
		go m.captureOutput(proc, stderr, "stderr")
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start process: %w", err)
	}

	proc.PID = cmd.Process.Pid
	proc.State = StateRunning

	// Write PID file
	pidFile := filepath.Join(m.pidDir, config.Name+".pid")
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(proc.PID)), 0644); err != nil {
		log.Printf("Warning: failed to write PID file: %v", err)
	}

	m.processes[config.Name] = proc

	// Monitor process in background
	go m.monitorProcess(config.Name, proc)

	log.Printf("[DirectManager] Started process %s (PID %d)", config.Name, proc.PID)

	if m.metrics != nil {
		m.metrics.RecordActivityExecution("DirectStart", "success")
	}

	return nil
}

// captureOutput captures stdout/stderr from a process
func (m *DirectManager) captureOutput(proc *ManagedProcess, reader io.ReadCloser, streamName string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()

		// Add timestamp
		timestamped := fmt.Sprintf("[%s] %s: %s",
			time.Now().Format("2006-01-02 15:04:05"), streamName, line)

		// Add to ring buffer
		proc.Output.Add(timestamped)

		// Write to log file if configured
		if proc.LogFile != nil {
			proc.LogFile.WriteString(timestamped + "\n")
		}
	}
}

// monitorProcess monitors a process and handles restart policy
func (m *DirectManager) monitorProcess(name string, proc *ManagedProcess) {
	// Wait for process to exit
	err := proc.Cmd.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			proc.ExitCode = exitErr.ExitCode()
			proc.State = StateFailed
			proc.Error = fmt.Sprintf("process exited with code %d", proc.ExitCode)
		} else {
			proc.State = StateFailed
			proc.Error = fmt.Sprintf("process error: %v", err)
		}
	} else {
		proc.ExitCode = 0
		proc.State = StateStopped
	}

	// Close log file
	if proc.LogFile != nil {
		proc.LogFile.Close()
		proc.LogFile = nil
	}

	// Remove PID file
	pidFile := filepath.Join(m.pidDir, name+".pid")
	os.Remove(pidFile)

	log.Printf("[DirectManager] Process %s exited (code: %d, state: %s)",
		name, proc.ExitCode, proc.State)

	// Handle restart policy
	shouldRestart := false
	switch proc.Config.RestartPolicy {
	case RestartAlways:
		shouldRestart = true
	case RestartOnFailure:
		shouldRestart = proc.ExitCode != 0
	case RestartNever:
		shouldRestart = false
	}

	if shouldRestart && (m.config.MaxRestarts == 0 || proc.RestartCount < m.config.MaxRestarts) {
		proc.RestartCount++

		// Apply restart delay
		delay := proc.Config.RestartDelay
		if delay == 0 {
			delay = 5 * time.Second
		}

		log.Printf("[DirectManager] Restarting %s in %v (attempt %d)",
			name, delay, proc.RestartCount)

		// Schedule restart
		time.AfterFunc(delay, func() {
			ctx := context.Background()
			if err := m.Start(ctx, proc.Config); err != nil {
				log.Printf("[DirectManager] Failed to restart %s: %v", name, err)
			}
		})

		if m.metrics != nil {
			m.metrics.RecordServiceRestart(name, "direct")
		}
	}
}

// Stop stops a process gracefully
func (m *DirectManager) Stop(ctx context.Context, name string) error {
	m.mu.Lock()
	proc, exists := m.processes[name]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("process %s not found", name)
	}
	m.mu.Unlock()

	proc.mu.Lock()
	if proc.State != StateRunning && proc.State != StateStarting {
		proc.mu.Unlock()
		return fmt.Errorf("process %s is not running", name)
	}

	proc.State = StateStopping
	proc.mu.Unlock()

	// Send SIGTERM
	if proc.Cmd.Process != nil {
		if err := proc.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return fmt.Errorf("failed to send SIGTERM: %w", err)
		}
	}

	// Wait for graceful shutdown with timeout
	done := make(chan bool)
	go func() {
		proc.Cmd.Wait()
		done <- true
	}()

	select {
	case <-done:
		log.Printf("[DirectManager] Process %s stopped gracefully", name)
	case <-time.After(10 * time.Second):
		// Force kill if still running
		log.Printf("[DirectManager] Process %s did not stop gracefully, killing", name)
		if proc.Cmd.Process != nil {
			proc.Cmd.Process.Kill()
		}
	}

	// Cancel context
	if proc.cancel != nil {
		proc.cancel()
	}

	if m.metrics != nil {
		m.metrics.RecordActivityExecution("DirectStop", "success")
	}

	return nil
}

// Restart restarts a process
func (m *DirectManager) Restart(ctx context.Context, name string) error {
	// Stop first
	if err := m.Stop(ctx, name); err != nil {
		// Process might already be stopped, continue
		log.Printf("[DirectManager] Stop returned error (continuing): %v", err)
	}

	// Wait a bit for cleanup
	time.Sleep(1 * time.Second)

	// Get config
	m.mu.RLock()
	proc, exists := m.processes[name]
	if !exists {
		m.mu.RUnlock()
		return fmt.Errorf("process %s not found", name)
	}
	config := proc.Config
	m.mu.RUnlock()

	// Start again
	return m.Start(ctx, config)
}

// GetStatus returns the current status of a process
func (m *DirectManager) GetStatus(ctx context.Context, name string) (*ProcessStatus, error) {
	m.mu.RLock()
	proc, exists := m.processes[name]
	m.mu.RUnlock()

	if !exists {
		return &ProcessStatus{
			Name:  name,
			State: StateStopped,
		}, nil
	}

	proc.mu.RLock()
	defer proc.mu.RUnlock()

	status := &ProcessStatus{
		Name:         name,
		State:        proc.State,
		PID:          proc.PID,
		StartTime:    proc.StartTime,
		RestartCount: proc.RestartCount,
		ExitCode:     proc.ExitCode,
		Error:        proc.Error,
		Healthy:      proc.State == StateRunning,
	}

	return status, nil
}

// GetOutput returns recent output lines
func (m *DirectManager) GetOutput(ctx context.Context, name string, lines int) ([]string, error) {
	m.mu.RLock()
	proc, exists := m.processes[name]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("process %s not found", name)
	}

	return proc.Output.GetLines(lines), nil
}

// Kill forcefully kills a process
func (m *DirectManager) Kill(ctx context.Context, name string) error {
	m.mu.RLock()
	proc, exists := m.processes[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("process %s not found", name)
	}

	if proc.Cmd.Process != nil {
		if err := proc.Cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

	if proc.cancel != nil {
		proc.cancel()
	}

	log.Printf("[DirectManager] Killed process %s (PID %d)", name, proc.PID)

	return nil
}

// List returns all managed processes
func (m *DirectManager) List(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.processes))
	for name := range m.processes {
		names = append(names, name)
	}

	return names, nil
}

// Close cleans up resources
func (m *DirectManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop all processes
	for name, proc := range m.processes {
		if proc.State == StateRunning || proc.State == StateStarting {
			log.Printf("[DirectManager] Stopping process %s on shutdown", name)
			if proc.Cmd.Process != nil {
				proc.Cmd.Process.Signal(syscall.SIGTERM)
			}
			if proc.cancel != nil {
				proc.cancel()
			}
		}

		if proc.LogFile != nil {
			proc.LogFile.Close()
		}
	}

	return nil
}

// Helper functions

func lookupUser(username, groupname string) (uint32, uint32, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to lookup user: %w", err)
	}

	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid UID: %w", err)
	}

	gid := uint64(0)
	if groupname != "" {
		g, err := user.LookupGroup(groupname)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to lookup group: %w", err)
		}
		gid, err = strconv.ParseUint(g.Gid, 10, 32)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid GID: %w", err)
		}
	} else {
		// Use user's primary group
		gid, err = strconv.ParseUint(u.Gid, 10, 32)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid GID: %w", err)
		}
	}

	return uint32(uid), uint32(gid), nil
}
