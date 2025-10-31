package main

import (
	"fmt"
	"time"

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
			// Service is healthy
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
		}

		// Wait before next health check
		// Using workflow.Sleep ensures this survives workflow/worker restarts
		if err := workflow.Sleep(ctx, config.WatchInterval); err != nil {
			logger.Info("Workflow cancelled or interrupted")
			return nil
		}
	}
}
