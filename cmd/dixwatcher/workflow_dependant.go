package main

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
)

// DependentServiceWorkflow - A service that depends on other services
// This workflow waits for dependencies, starts the service, checks sync if needed,
// and propagates readiness signals
func DependentServiceWorkflow(ctx workflow.Context, config DependentServiceConfig) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("DependentServiceWorkflow started",
		"service", config.NodeConfig.Name,
		"dependencies", len(config.Dependencies))

	// Wait for all dependencies to be ready
	for _, dep := range config.Dependencies {
		logger.Info("Waiting for dependency",
			"service", config.NodeConfig.Name,
			"dependency", dep.WorkflowID)

		// Set up signal channels for each dependency
		for _, signalName := range dep.SignalNames {
			signalChan := workflow.GetSignalChannel(ctx, signalName)

			// Wait for ready signal with timeout
			selector := workflow.NewSelector(ctx)

			var ready bool
			selector.AddReceive(signalChan, func(c workflow.ReceiveChannel, more bool) {
				c.Receive(ctx, &ready)
			})

			// Add timeout
			timeoutTimer := workflow.NewTimer(ctx, time.Duration(dep.TimeoutHours)*time.Hour)
			selector.AddFuture(timeoutTimer, func(f workflow.Future) {
				logger.Error("Dependency timeout",
					"service", config.NodeConfig.Name,
					"dependency", dep.WorkflowID)
			})

			selector.Select(ctx)

			if ready {
				logger.Info("Dependency ready",
					"service", config.NodeConfig.Name,
					"dependency", dep.WorkflowID)
				break
			}
		}
	}

	logger.Info("All dependencies satisfied, starting service",
		"service", config.NodeConfig.Name)

	// All dependencies ready - start the node workflow as a child
	// This will handle systemd service management and sync checking
	childOptions := workflow.ChildWorkflowOptions{
		WorkflowID: fmt.Sprintf("dependent-%s", config.NodeConfig.Name),
	}
	childCtx := workflow.WithChildOptions(ctx, childOptions)

	err := workflow.ExecuteChildWorkflow(childCtx, NodeWorkflow, config.NodeConfig).Get(ctx, nil)
	return err
}
