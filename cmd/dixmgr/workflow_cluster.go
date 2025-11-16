package main

import (
	"fmt"

	"go.temporal.io/sdk/workflow"
)

// Example 1: ClusterWorkflow - Manages N redundant instances of a service
// Use case: Run 2 relay chain nodes for redundancy
func ClusterWorkflowExample(ctx workflow.Context, config ClusterWorkflowConfig) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("ClusterWorkflow started", "cluster", config.Name, "replicas", config.ReplicaCount)

	// Start N child workflows (one per replica)
	childWorkflows := make([]workflow.ChildWorkflowFuture, 0, config.ReplicaCount)

	for i := 0; i < config.ReplicaCount; i++ {
		nodeConfig := config.Nodes[i]

		childOptions := workflow.ChildWorkflowOptions{
			WorkflowID: fmt.Sprintf("cluster-%s-node-%d", config.Name, i),
		}
		childCtx := workflow.WithChildOptions(ctx, childOptions)

		child := workflow.ExecuteChildWorkflow(childCtx, NodeWorkflow, nodeConfig)
		childWorkflows = append(childWorkflows, child)

		logger.Info("Started replica", "cluster", config.Name, "replica", i, "workflowID", nodeConfig.Name)
	}

	// Monitor cluster health via signals from child workflows
	healthSignal := workflow.GetSignalChannel(ctx, "NodeHealthUpdate")

	healthyNodes := make(map[string]bool)

	for {
		var update NodeHealthStatus
		healthSignal.Receive(ctx, &update)

		healthyNodes[update.NodeID] = update.IsHealthy

		// Count healthy nodes
		healthyCount := 0
		for _, healthy := range healthyNodes {
			if healthy {
				healthyCount++
			}
		}

		// Check if we meet quorum
		if healthyCount >= config.Quorum {
			logger.Info("Cluster is healthy",
				"cluster", config.Name,
				"healthyNodes", healthyCount,
				"quorum", config.Quorum)
		} else {
			logger.Warn("Cluster below quorum",
				"cluster", config.Name,
				"healthyNodes", healthyCount,
				"quorum", config.Quorum)
		}
	}
}
