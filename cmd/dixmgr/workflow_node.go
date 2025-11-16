package main

import (
	"fmt"
	"math/rand"
	"time"

	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// NodeHealthStatus represents the health status that workflows can signal
type NodeHealthStatus struct {
	NodeID    string
	IsHealthy bool
	Timestamp time.Time
	Message   string
}

// NodeWorkflow manages a single systemd service with automatic health checks and restarts
// This workflow runs indefinitely until cancelled, continuously monitoring the service
func NodeWorkflow(ctx workflow.Context, config NodeWorkflowConfig) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("NodeWorkflow started",
		"name", config.Name,
		"unit", config.SystemdUnit,
		"watchInterval", config.WatchInterval,
		"maxRestarts", config.MaxRestarts)

	// Configure activity options
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    60 * time.Second,
			MaximumAttempts:    3, // Activities can retry a few times
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	// State variables (persisted across failures by Temporal)
	restartCount := 0
	consecutiveFailures := 0
	lastHealthy := workflow.Now(ctx)

	// Signal parent about initial state
	if config.ParentWorkflowID != "" {
		_ = workflow.SignalExternalWorkflow(ctx, config.ParentWorkflowID, "",
			"NodeHealthUpdate", NodeHealthStatus{
				NodeID:    config.Name,
				IsHealthy: false,
				Timestamp: workflow.Now(ctx),
				Message:   "Starting up",
			})
	}

	// Track readiness state
	readySignalSent := false

	// Main monitoring loop
	for {
		// Check service health
		var status *SystemdServiceStatus
		err := workflow.ExecuteActivity(ctx, "CheckSystemdServiceActivity", config.SystemdUnit).Get(ctx, &status)

		if err != nil {
			// Activity failed (systemd not reachable, etc.)
			logger.Error("Health check activity failed",
				"service", config.SystemdUnit,
				"error", err)
			consecutiveFailures++

			// Signal parent about unhealthy state
			if config.ParentWorkflowID != "" {
				_ = workflow.SignalExternalWorkflow(ctx, config.ParentWorkflowID, "",
					"NodeHealthUpdate", NodeHealthStatus{
						NodeID:    config.Name,
						IsHealthy: false,
						Timestamp: workflow.Now(ctx),
						Message:   fmt.Sprintf("Health check failed: %v", err),
					})
			}

		} else if !status.IsActive {
			// Service is not active
			consecutiveFailures++
			logger.Warn("Service is not active",
				"service", config.SystemdUnit,
				"activeState", status.ActiveState,
				"subState", status.SubState,
				"consecutiveFailures", consecutiveFailures)

			// Signal parent about unhealthy state
			if config.ParentWorkflowID != "" {
				_ = workflow.SignalExternalWorkflow(ctx, config.ParentWorkflowID, "",
					"NodeHealthUpdate", NodeHealthStatus{
						NodeID:    config.Name,
						IsHealthy: false,
						Timestamp: workflow.Now(ctx),
						Message:   fmt.Sprintf("Service inactive: %s", status.ActiveState),
					})
			}

			// Attempt restart if under max restarts
			if restartCount < config.MaxRestarts {
				restartCount++

				// Apply exponential backoff
				if restartCount > 1 {
					backoffDuration := time.Duration(restartCount) * config.RestartBackoff
					logger.Info("Applying restart backoff",
						"service", config.SystemdUnit,
						"backoff", backoffDuration,
						"attempt", restartCount)
					_ = workflow.Sleep(ctx, backoffDuration)
				}

				// Restart the service
				logger.Info("Attempting restart",
					"service", config.SystemdUnit,
					"attempt", restartCount,
					"maxRestarts", config.MaxRestarts)

				err := workflow.ExecuteActivity(ctx, "RestartSystemdServiceActivity", config.SystemdUnit).Get(ctx, nil)
				if err != nil {
					logger.Error("Restart failed",
						"service", config.SystemdUnit,
						"attempt", restartCount,
						"error", err)
				} else {
					logger.Info("Restart successful",
						"service", config.SystemdUnit,
						"attempt", restartCount)
				}

			} else {
				logger.Error("Max restarts reached, giving up",
					"service", config.SystemdUnit,
					"maxRestarts", config.MaxRestarts)

				// Signal parent about permanent failure
				if config.ParentWorkflowID != "" {
					_ = workflow.SignalExternalWorkflow(ctx, config.ParentWorkflowID, "",
						"NodeFailed", NodeHealthStatus{
							NodeID:    config.Name,
							IsHealthy: false,
							Timestamp: workflow.Now(ctx),
							Message:   "Max restarts exceeded",
						})
				}
			}

		} else {
			// Service is healthy (systemd active)
			if consecutiveFailures > 0 {
				logger.Info("Service recovered",
					"service", config.SystemdUnit,
					"downtime", workflow.Now(ctx).Sub(lastHealthy))
			}

			consecutiveFailures = 0
			restartCount = 0 // Reset restart count on healthy state
			lastHealthy = workflow.Now(ctx)

			// Signal parent about healthy state
			if config.ParentWorkflowID != "" {
				_ = workflow.SignalExternalWorkflow(ctx, config.ParentWorkflowID, "",
					"NodeHealthUpdate", NodeHealthStatus{
						NodeID:    config.Name,
						IsHealthy: true,
						Timestamp: workflow.Now(ctx),
						Message:   "Healthy",
					})
			}

			// Check blockchain sync status if required and not yet signaled ready
			if config.CheckSync && !readySignalSent {
				synced, err := checkNodeSync(ctx, config, logger)
				if err != nil {
					logger.Warn("Sync check failed", "service", config.Name, "error", err)
				} else if synced {
					logger.Info("Node is synced and ready", "service", config.Name)
					readySignalSent = emitReadySignal(ctx, config, logger)
				} else {
					logger.Info("Node is syncing", "service", config.Name)
				}
			} else if !config.CheckSync && !readySignalSent {
				// No sync check required, emit ready signal immediately
				logger.Info("Service ready (no sync check required)", "service", config.Name)
				readySignalSent = emitReadySignal(ctx, config, logger)
			}
		}

		// Wait before next health check
		// Using workflow.Sleep ensures this survives workflow/worker restarts
		if err := workflow.Sleep(ctx, config.WatchInterval); err != nil {
			logger.Info("Workflow cancelled or interrupted")
			return nil
		}
	}
}

// checkNodeSync checks if a blockchain node has completed syncing
func checkNodeSync(ctx workflow.Context, config NodeWorkflowConfig, logger log.Logger) (bool, error) {
	// Configure activity options for sync check with retries
	syncActivityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    3,
		},
	}
	syncCtx := workflow.WithActivityOptions(ctx, syncActivityOptions)

	var synced bool
	err := workflow.ExecuteActivity(syncCtx, "CheckNodeSyncActivity", config.RPCEndpoint, config.RPCPort).Get(syncCtx, &synced)
	if err != nil {
		return false, fmt.Errorf("sync check activity failed: %w", err)
	}

	return synced, nil
}

// emitReadySignal sends the ready signal to the parent workflow
// Returns true if signal was sent successfully
func emitReadySignal(ctx workflow.Context, config NodeWorkflowConfig, logger log.Logger) bool {
	if config.ReadySignal == "" || config.ParentWorkflowID == "" {
		logger.Info("Ready signal not configured", "service", config.Name)
		return true // Consider it sent if not configured
	}

	err := workflow.SignalExternalWorkflow(ctx, config.ParentWorkflowID, "", config.ReadySignal, true).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to send ready signal",
			"service", config.Name,
			"signal", config.ReadySignal,
			"parent", config.ParentWorkflowID,
			"error", err)
		return false
	}

	logger.Info("Ready signal sent",
		"service", config.Name,
		"signal", config.ReadySignal,
		"parent", config.ParentWorkflowID)
	return true
}

// calculateBackoffWithJitter calculates exponential backoff with jitter
func calculateBackoffWithJitter(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	// Exponential backoff: baseDelay * 2^attempt
	delay := baseDelay * time.Duration(1<<uint(attempt))
	if delay > maxDelay {
		delay = maxDelay
	}

	// Add 10-20% jitter
	jitter := time.Duration(float64(delay) * (0.1 + rand.Float64()*0.1))
	return delay + jitter
}
