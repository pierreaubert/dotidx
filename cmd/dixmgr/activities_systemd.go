package main

import (
	"context"
	"fmt"
	"log"
	"time"
)

// SystemdServiceStatus represents the status of a systemd service
type SystemdServiceStatus struct {
	IsActive    bool
	ActiveState string
	SubState    string
	LoadState   string
}

// CheckSystemdServiceActivity checks if a systemd service is running and healthy
func (a *Activities) CheckSystemdServiceActivity(ctx context.Context, unitName string) (*SystemdServiceStatus, error) {
	start := time.Now()
	log.Printf("[Activity] Checking systemd service: %s", unitName)

	props, err := a.dbusConn.GetUnitPropertiesContext(ctx, unitName)
	if err != nil {
		// Record metrics
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("CheckSystemdService", "error")
			a.metrics.RecordActivityError("CheckSystemdService", "dbus_error")
		}
		return nil, fmt.Errorf("failed to get properties for %s: %w", unitName, err)
	}

	activeState, ok := props["ActiveState"].(string)
	if !ok {
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("CheckSystemdService", "error")
			a.metrics.RecordActivityError("CheckSystemdService", "parse_error")
		}
		return nil, fmt.Errorf("ActiveState for %s is not a string or not found", unitName)
	}

	subState, _ := props["SubState"].(string)
	loadState, _ := props["LoadState"].(string)

	status := &SystemdServiceStatus{
		IsActive:    activeState == "active",
		ActiveState: activeState,
		SubState:    subState,
		LoadState:   loadState,
	}

	log.Printf("[Activity] Service %s status: ActiveState=%s, SubState=%s, LoadState=%s",
		unitName, status.ActiveState, status.SubState, status.LoadState)

	// Record metrics
	if a.metrics != nil {
		a.metrics.RecordActivityExecution("CheckSystemdService", "success")
		a.metrics.RecordActivityDuration("CheckSystemdService", time.Since(start))

		// Determine service type from unit name
		serviceType := "service"
		if len(unitName) > 0 {
			// Simple heuristic: extract service type from unit name
			serviceType = unitName
		}
		a.metrics.RecordServiceHealth(unitName, serviceType, "", status.IsActive)
	}

	// Evaluate alert rules
	if a.alertEngine != nil {
		a.alertEngine.EvaluateServiceStatus(ctx, unitName, status)
	}

	// Check resource usage if enabled
	if a.enableResourceMonitoring && status.IsActive {
		// Capture serviceType for the goroutine
		capturedServiceType := "service"
		if len(unitName) > 0 {
			capturedServiceType = unitName
		}

		go func() {
			usage, err := a.CheckResourceUsageActivity(ctx, unitName)
			if err == nil && usage != nil {
				if a.metrics != nil {
					a.metrics.RecordResourceUsage(unitName, capturedServiceType,
						usage.CPUPercent, float64(usage.MemoryBytes),
						usage.DiskReadBPS, usage.DiskWriteBPS)
				}
				if a.alertEngine != nil {
					a.alertEngine.EvaluateResourceUsage(ctx, unitName, usage)
				}
			}
		}()
	}

	return status, nil
}

// StartSystemdServiceActivity starts a systemd service
func (a *Activities) StartSystemdServiceActivity(ctx context.Context, unitName string) error {
	if !a.executeMode {
		log.Printf("[Activity] [DRY-RUN] Would start systemd service: %s", unitName)
		return nil
	}

	log.Printf("[Activity] Starting systemd service: %s", unitName)

	reschan := make(chan string)
	_, err := a.dbusConn.StartUnitContext(ctx, unitName, "replace", reschan)
	if err != nil {
		return fmt.Errorf("failed to start service %s: %w", unitName, err)
	}

	// Wait for result with timeout
	select {
	case result := <-reschan:
		if result == "done" {
			log.Printf("[Activity] Successfully started service: %s", unitName)
			return nil
		}
		return fmt.Errorf("start operation for %s finished with result: %s", unitName, result)
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for start operation on %s: %w", unitName, ctx.Err())
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for start operation on %s", unitName)
	}
}

// StopSystemdServiceActivity stops a systemd service
func (a *Activities) StopSystemdServiceActivity(ctx context.Context, unitName string) error {
	if !a.executeMode {
		log.Printf("[Activity] [DRY-RUN] Would stop systemd service: %s", unitName)
		return nil
	}

	log.Printf("[Activity] Stopping systemd service: %s", unitName)

	reschan := make(chan string)
	_, err := a.dbusConn.StopUnitContext(ctx, unitName, "replace", reschan)
	if err != nil {
		return fmt.Errorf("failed to stop service %s: %w", unitName, err)
	}

	// Wait for result with timeout
	select {
	case result := <-reschan:
		if result == "done" {
			log.Printf("[Activity] Successfully stopped service: %s", unitName)
			return nil
		}
		return fmt.Errorf("stop operation for %s finished with result: %s", unitName, result)
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for stop operation on %s: %w", unitName, ctx.Err())
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for stop operation on %s", unitName)
	}
}

// RestartSystemdServiceActivity restarts a systemd service
func (a *Activities) RestartSystemdServiceActivity(ctx context.Context, unitName string) error {
	start := time.Now()

	if !a.executeMode {
		log.Printf("[Activity] [DRY-RUN] Would restart systemd service: %s", unitName)
		return nil
	}

	log.Printf("[Activity] Restarting systemd service: %s", unitName)

	reschan := make(chan string)
	_, err := a.dbusConn.RestartUnitContext(ctx, unitName, "replace", reschan)
	if err != nil {
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("RestartSystemdService", "error")
			a.metrics.RecordActivityError("RestartSystemdService", "restart_failed")
		}
		return fmt.Errorf("failed to restart service %s: %w", unitName, err)
	}

	// Wait for result with timeout
	select {
	case result := <-reschan:
		if result == "done" {
			log.Printf("[Activity] Successfully restarted service: %s", unitName)

			// Record metrics
			if a.metrics != nil {
				a.metrics.RecordActivityExecution("RestartSystemdService", "success")
				a.metrics.RecordActivityDuration("RestartSystemdService", time.Since(start))
				a.metrics.RecordServiceRestart(unitName, "service")
			}

			return nil
		}
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("RestartSystemdService", "failed")
		}
		return fmt.Errorf("restart operation for %s finished with result: %s", unitName, result)
	case <-ctx.Done():
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("RestartSystemdService", "timeout")
		}
		return fmt.Errorf("timeout waiting for restart operation on %s: %w", unitName, ctx.Err())
	case <-time.After(30 * time.Second):
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("RestartSystemdService", "timeout")
		}
		return fmt.Errorf("timeout waiting for restart operation on %s", unitName)
	}
}
