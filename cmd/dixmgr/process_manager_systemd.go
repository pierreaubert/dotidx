package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
)

// SystemdManager manages processes via systemd
type SystemdManager struct {
	conn    *dbus.Conn
	metrics *MetricsCollector
	config  ProcessManagerConfig
}

// NewSystemdManager creates a new systemd-based process manager
func NewSystemdManager(config ProcessManagerConfig, metrics *MetricsCollector) (*SystemdManager, error) {
	conn, err := dbus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to D-Bus: %w", err)
	}

	return &SystemdManager{
		conn:    conn,
		metrics: metrics,
		config:  config,
	}, nil
}

// Name returns the manager type
func (m *SystemdManager) Name() string {
	return "systemd"
}

// Start starts a systemd service
func (m *SystemdManager) Start(ctx context.Context, config ProcessConfig) error {
	unitName := config.Name
	if !hasSystemdSuffix(unitName) {
		unitName = unitName + ".service"
	}

	reschan := make(chan string)
	_, err := m.conn.StartUnitContext(ctx, unitName, "replace", reschan)
	if err != nil {
		if m.metrics != nil {
			m.metrics.RecordActivityError("SystemdStart", "start_failed")
		}
		return fmt.Errorf("failed to start %s: %w", unitName, err)
	}

	// Wait for result with timeout
	select {
	case result := <-reschan:
		if result == "done" {
			if m.metrics != nil {
				m.metrics.RecordActivityExecution("SystemdStart", "success")
			}
			return nil
		}
		return fmt.Errorf("start operation finished with result: %s", result)
	case <-ctx.Done():
		return fmt.Errorf("start operation timed out: %w", ctx.Err())
	case <-time.After(30 * time.Second):
		return fmt.Errorf("start operation timed out")
	}
}

// Stop stops a systemd service
func (m *SystemdManager) Stop(ctx context.Context, name string) error {
	unitName := name
	if !hasSystemdSuffix(unitName) {
		unitName = unitName + ".service"
	}

	reschan := make(chan string)
	_, err := m.conn.StopUnitContext(ctx, unitName, "replace", reschan)
	if err != nil {
		if m.metrics != nil {
			m.metrics.RecordActivityError("SystemdStop", "stop_failed")
		}
		return fmt.Errorf("failed to stop %s: %w", unitName, err)
	}

	select {
	case result := <-reschan:
		if result == "done" {
			if m.metrics != nil {
				m.metrics.RecordActivityExecution("SystemdStop", "success")
			}
			return nil
		}
		return fmt.Errorf("stop operation finished with result: %s", result)
	case <-ctx.Done():
		return fmt.Errorf("stop operation timed out: %w", ctx.Err())
	case <-time.After(30 * time.Second):
		return fmt.Errorf("stop operation timed out")
	}
}

// Restart restarts a systemd service
func (m *SystemdManager) Restart(ctx context.Context, name string) error {
	unitName := name
	if !hasSystemdSuffix(unitName) {
		unitName = unitName + ".service"
	}

	reschan := make(chan string)
	_, err := m.conn.RestartUnitContext(ctx, unitName, "replace", reschan)
	if err != nil {
		if m.metrics != nil {
			m.metrics.RecordActivityError("SystemdRestart", "restart_failed")
		}
		return fmt.Errorf("failed to restart %s: %w", unitName, err)
	}

	select {
	case result := <-reschan:
		if result == "done" {
			if m.metrics != nil {
				m.metrics.RecordActivityExecution("SystemdRestart", "success")
				m.metrics.RecordServiceRestart(unitName, "service")
			}
			return nil
		}
		return fmt.Errorf("restart operation finished with result: %s", result)
	case <-ctx.Done():
		return fmt.Errorf("restart operation timed out: %w", ctx.Err())
	case <-time.After(30 * time.Second):
		return fmt.Errorf("restart operation timed out")
	}
}

// GetStatus returns the status of a systemd service
func (m *SystemdManager) GetStatus(ctx context.Context, name string) (*ProcessStatus, error) {
	unitName := name
	if !hasSystemdSuffix(unitName) {
		unitName = unitName + ".service"
	}

	props, err := m.conn.GetUnitPropertiesContext(ctx, unitName)
	if err != nil {
		if m.metrics != nil {
			m.metrics.RecordActivityError("SystemdStatus", "get_props_failed")
		}
		return nil, fmt.Errorf("failed to get properties for %s: %w", unitName, err)
	}

	activeState, _ := props["ActiveState"].(string)
	subState, _ := props["SubState"].(string)
	mainPID, _ := props["MainPID"].(uint32)

	status := &ProcessStatus{
		Name:  name,
		State: mapSystemdState(activeState, subState),
		PID:   int(mainPID),
	}

	// Try to get additional info
	if execMainStartTimestamp, ok := props["ExecMainStartTimestamp"].(uint64); ok && execMainStartTimestamp > 0 {
		status.StartTime = time.Unix(int64(execMainStartTimestamp/1000000), 0)
	}

	if exitCode, ok := props["ExecMainCode"].(int32); ok {
		status.ExitCode = int(exitCode)
	}

	if nRestarts, ok := props["NRestarts"].(uint32); ok {
		status.RestartCount = int(nRestarts)
	}

	status.Healthy = status.State == StateRunning

	if m.metrics != nil {
		m.metrics.RecordActivityExecution("SystemdStatus", "success")
	}

	return status, nil
}

// GetOutput returns recent output (from journald)
func (m *SystemdManager) GetOutput(ctx context.Context, name string, lines int) ([]string, error) {
	// Would need to use journalctl API or exec journalctl command
	// For now, return empty - this is primarily useful for DirectManager
	return []string{}, nil
}

// Kill forcefully kills a systemd service
func (m *SystemdManager) Kill(ctx context.Context, name string) error {
	unitName := name
	if !hasSystemdSuffix(unitName) {
		unitName = unitName + ".service"
	}

	// Use KillUnit to send SIGKILL
	m.conn.KillUnitContext(ctx, unitName, int32(9)) // SIGKILL
	// Note: KillUnit may succeed even if unit is already stopped
	log.Printf("[SystemdManager] Killed service: %s", unitName)

	return nil
}

// List returns all managed services
func (m *SystemdManager) List(ctx context.Context) ([]string, error) {
	units, err := m.conn.ListUnitsContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list units: %w", err)
	}

	services := make([]string, 0)
	for _, unit := range units {
		if strings.HasSuffix(unit.Name, ".service") {
			services = append(services, strings.TrimSuffix(unit.Name, ".service"))
		}
	}

	return services, nil
}

// Close closes the D-Bus connection
func (m *SystemdManager) Close() error {
	if m.conn != nil {
		m.conn.Close()
	}
	return nil
}

// Helper functions

func hasSystemdSuffix(name string) bool {
	return strings.HasSuffix(name, ".service") ||
		strings.HasSuffix(name, ".socket") ||
		strings.HasSuffix(name, ".timer")
}

func mapSystemdState(activeState, subState string) ProcessState {
	switch activeState {
	case "active":
		if subState == "running" {
			return StateRunning
		}
		return StateStarting
	case "inactive":
		return StateStopped
	case "activating":
		return StateStarting
	case "deactivating":
		return StateStopping
	case "failed":
		return StateFailed
	default:
		return StateUnknown
	}
}
