package main

import (
	"context"
	"fmt"
	"log"
	"time"
)

// StartProcessActivity starts a process using the configured process manager
func (a *Activities) StartProcessActivity(ctx context.Context, config ProcessConfig) error {
	if !a.executeMode {
		log.Printf("[Activity] [DRY-RUN] Would start process: %s", config.Name)
		return nil
	}

	start := time.Now()
	log.Printf("[Activity] Starting process: %s", config.Name)

	// Use circuit breaker if available
	if a.circuitBreakers != nil {
		cb := a.circuitBreakers.GetOrCreate(config.Name)
		err := cb.Call(ctx, func() error {
			return a.processManager.Start(ctx, config)
		})

		if err != nil {
			if a.metrics != nil {
				a.metrics.RecordActivityExecution("StartProcess", "error")
				a.metrics.RecordActivityError("StartProcess", "start_failed")
			}
			return err
		}
	} else {
		if err := a.processManager.Start(ctx, config); err != nil {
			if a.metrics != nil {
				a.metrics.RecordActivityExecution("StartProcess", "error")
				a.metrics.RecordActivityError("StartProcess", "start_failed")
			}
			return err
		}
	}

	if a.metrics != nil {
		a.metrics.RecordActivityExecution("StartProcess", "success")
		a.metrics.RecordActivityDuration("StartProcess", time.Since(start))
	}

	return nil
}

// StopProcessActivity stops a process
func (a *Activities) StopProcessActivity(ctx context.Context, name string) error {
	if !a.executeMode {
		log.Printf("[Activity] [DRY-RUN] Would stop process: %s", name)
		return nil
	}

	start := time.Now()
	log.Printf("[Activity] Stopping process: %s", name)

	if err := a.processManager.Stop(ctx, name); err != nil {
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("StopProcess", "error")
			a.metrics.RecordActivityError("StopProcess", "stop_failed")
		}
		return err
	}

	if a.metrics != nil {
		a.metrics.RecordActivityExecution("StopProcess", "success")
		a.metrics.RecordActivityDuration("StopProcess", time.Since(start))
	}

	return nil
}

// RestartProcessActivity restarts a process
func (a *Activities) RestartProcessActivity(ctx context.Context, name string) error {
	if !a.executeMode {
		log.Printf("[Activity] [DRY-RUN] Would restart process: %s", name)
		return nil
	}

	start := time.Now()
	log.Printf("[Activity] Restarting process: %s", name)

	if err := a.processManager.Restart(ctx, name); err != nil {
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("RestartProcess", "error")
			a.metrics.RecordActivityError("RestartProcess", "restart_failed")
		}
		return err
	}

	if a.metrics != nil {
		a.metrics.RecordActivityExecution("RestartProcess", "success")
		a.metrics.RecordActivityDuration("RestartProcess", time.Since(start))
		a.metrics.RecordServiceRestart(name, "process")
	}

	// Record to health history
	if a.healthHistory != nil {
		a.healthHistory.RecordRestart(name, "workflow-initiated", true)
	}

	return nil
}

// CheckProcessActivity checks the status of a process
func (a *Activities) CheckProcessActivity(ctx context.Context, name string) (*ProcessStatus, error) {
	start := time.Now()
	log.Printf("[Activity] Checking process: %s", name)

	status, err := a.processManager.GetStatus(ctx, name)
	if err != nil {
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("CheckProcess", "error")
			a.metrics.RecordActivityError("CheckProcess", "status_failed")
		}
		return nil, err
	}

	if a.metrics != nil {
		a.metrics.RecordActivityExecution("CheckProcess", "success")
		a.metrics.RecordActivityDuration("CheckProcess", time.Since(start))

		// Record service health
		a.metrics.RecordServiceHealth(name, "process", "", status.Healthy)
	}

	// Evaluate alert rules
	if a.alertEngine != nil {
		// Convert ProcessStatus to SystemdServiceStatus for compatibility
		sysStatus := &SystemdServiceStatus{
			IsActive:    status.State == StateRunning,
			ActiveState: string(status.State),
			SubState:    "",
			LoadState:   "",
		}
		a.alertEngine.EvaluateServiceStatus(ctx, name, sysStatus)
	}

	// Record to health history
	if a.healthHistory != nil {
		event := HealthEvent{
			Service:      name,
			ServiceType:  "process",
			IsHealthy:    status.Healthy,
			ActiveState:  string(status.State),
			RestartCount: status.RestartCount,
			ErrorMessage: status.Error,
		}

		// Try to get resource usage for this process
		if a.enableResourceMonitoring && status.PID > 0 {
			// We can get resource usage using the PID
			// For now, record basic info
			event.CPUPercent = status.CPUPercent
			event.MemoryBytes = status.MemoryBytes
		}

		a.healthHistory.RecordHealthEvent(event)
	}

	return status, nil
}

// GetProcessOutputActivity retrieves recent output from a process
func (a *Activities) GetProcessOutputActivity(ctx context.Context, name string, lines int) ([]string, error) {
	if lines == 0 {
		lines = 100
	}

	output, err := a.processManager.GetOutput(ctx, name, lines)
	if err != nil {
		return nil, fmt.Errorf("failed to get output for %s: %w", name, err)
	}

	return output, nil
}

// KillProcessActivity forcefully kills a process
func (a *Activities) KillProcessActivity(ctx context.Context, name string) error {
	if !a.executeMode {
		log.Printf("[Activity] [DRY-RUN] Would kill process: %s", name)
		return nil
	}

	log.Printf("[Activity] Killing process: %s", name)

	if err := a.processManager.Kill(ctx, name); err != nil {
		if a.metrics != nil {
			a.metrics.RecordActivityExecution("KillProcess", "error")
			a.metrics.RecordActivityError("KillProcess", "kill_failed")
		}
		return err
	}

	if a.metrics != nil {
		a.metrics.RecordActivityExecution("KillProcess", "success")
	}

	return nil
}

// ListProcessesActivity lists all managed processes
func (a *Activities) ListProcessesActivity(ctx context.Context) ([]string, error) {
	processes, err := a.processManager.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list processes: %w", err)
	}

	return processes, nil
}
