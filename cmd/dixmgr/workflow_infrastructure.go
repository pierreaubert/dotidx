package main

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
)

// InfrastructureWorkflow - Root orchestrator for the entire dotidx infrastructure
// Orchestrates relay chains → parachains → sidecars → nginx → app services
func InfrastructureWorkflow(ctx workflow.Context, input InfrastructureWorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("InfrastructureWorkflow started", "relays", len(input.RelayPlans))

	// Track all expected ready signals
	var allSidecarSignals []string

	// Phase 1: Start all relay chains and their parachains
	for _, relayPlan := range input.RelayPlans {
		logger.Info("Starting relay chain", "relay", relayPlan.RelayID)

		// Start relay chain node
		relayWorkflowID := WorkflowIDNodeRelay(relayPlan.RelayID)
		relayCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID: relayWorkflowID,
		})
		workflow.ExecuteChildWorkflow(relayCtx, NodeWorkflow, relayPlan.Node)

		// Wait for relay chain to be ready
		relayReadySignal := ReadySignalRelay(relayPlan.RelayID)
		relayReadyChan := workflow.GetSignalChannel(ctx, relayReadySignal)
		var relayReady bool
		relayReadyChan.Receive(ctx, &relayReady)
		logger.Info("Relay chain ready", "relay", relayPlan.RelayID)

		// Start parachains attached to this relay
		for _, paraPlan := range relayPlan.Parachains {
			logger.Info("Starting parachain",
				"relay", relayPlan.RelayID,
				"chain", paraPlan.ChainID)

			// Start parachain node (depends on relay)
			paraWorkflowID := WorkflowIDNodePara(relayPlan.RelayID, paraPlan.ChainID)
			paraCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
				WorkflowID: paraWorkflowID,
			})

			// Parachain depends on relay being ready
			paraDependency := DependentServiceConfig{
				NodeConfig: paraPlan.Node,
				Dependencies: []DependencyInfo{
					{
						WorkflowID:   WorkflowIDInfra(),
						SignalNames:  []string{relayReadySignal},
						RequiredAny:  false,
						TimeoutHours: 24,
					},
				},
			}
			workflow.ExecuteChildWorkflow(paraCtx, DependentServiceWorkflow, paraDependency)

			// Wait for parachain to be ready
			paraReadySignal := ReadySignalPara(relayPlan.RelayID, paraPlan.ChainID)
			paraReadyChan := workflow.GetSignalChannel(ctx, paraReadySignal)
			var paraReady bool
			paraReadyChan.Receive(ctx, &paraReady)
			logger.Info("Parachain ready",
				"relay", relayPlan.RelayID,
				"chain", paraPlan.ChainID)

			// Start N sidecar instances for this parachain
			for i := 0; i < paraPlan.SidecarCount; i++ {
				logger.Info("Starting sidecar",
					"relay", relayPlan.RelayID,
					"chain", paraPlan.ChainID,
					"index", i)

				sidecarConfig := NodeWorkflowConfig{
					Name:             fmt.Sprintf("Sidecar-%s-%s-%d", relayPlan.RelayID, paraPlan.ChainID, i),
					SystemdUnit:      fmt.Sprintf("sidecar@%s-%s-%d.service", relayPlan.RelayID, paraPlan.ChainID, i),
					ServiceName:      fmt.Sprintf("%s-%d", paraPlan.SidecarServiceName, i),
					CheckSync:        false, // Sidecars don't need sync check
					ReadySignal:      ReadySignalSidecar(relayPlan.RelayID, paraPlan.ChainID, i),
					ParentWorkflowID: WorkflowIDInfra(),
					WatchInterval:    30 * time.Second,
					MaxRestarts:      5,
					RestartBackoff:   10 * time.Second,
				}

				sidecarWorkflowID := WorkflowIDSidecar(relayPlan.RelayID, paraPlan.ChainID, i)
				sidecarCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
					WorkflowID: sidecarWorkflowID,
				})

				// Sidecar depends on parachain being ready
				sidecarDependency := DependentServiceConfig{
					NodeConfig: sidecarConfig,
					Dependencies: []DependencyInfo{
						{
							WorkflowID:   WorkflowIDInfra(),
							SignalNames:  []string{paraReadySignal},
							RequiredAny:  false,
							TimeoutHours: 24,
						},
					},
				}
				workflow.ExecuteChildWorkflow(sidecarCtx, DependentServiceWorkflow, sidecarDependency)

				// Track sidecar signal for nginx dependency
				allSidecarSignals = append(allSidecarSignals, ReadySignalSidecar(relayPlan.RelayID, paraPlan.ChainID, i))
			}
		}
	}

	// Phase 2: Wait for all sidecars to be ready
	logger.Info("Waiting for all sidecars", "count", len(allSidecarSignals))
	sidecarReadyCount := 0
	for _, sidecarSignal := range allSidecarSignals {
		sidecarChan := workflow.GetSignalChannel(ctx, sidecarSignal)
		var ready bool
		sidecarChan.Receive(ctx, &ready)
		sidecarReadyCount++
		logger.Info("Sidecar ready", "signal", sidecarSignal, "progress", fmt.Sprintf("%d/%d", sidecarReadyCount, len(allSidecarSignals)))
	}

	logger.Info("All sidecars ready, starting nginx")

	// Phase 3: Start nginx (depends on all sidecars)
	nginxConfig := NodeWorkflowConfig{
		Name:             "Nginx",
		SystemdUnit:      "dix-nginx.service",
		ServiceName:      input.NginxService,
		CheckSync:        false,
		ReadySignal:      ReadySignalSvc(input.NginxService),
		ParentWorkflowID: WorkflowIDInfra(),
		WatchInterval:    30 * time.Second,
		MaxRestarts:      5,
		RestartBackoff:   10 * time.Second,
	}

	nginxWorkflowID := WorkflowIDSvc(input.NginxService)
	nginxCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: nginxWorkflowID,
	})

	nginxDependency := DependentServiceConfig{
		NodeConfig: nginxConfig,
		Dependencies: []DependencyInfo{
			{
				WorkflowID:   WorkflowIDInfra(),
				SignalNames:  allSidecarSignals,
				RequiredAny:  false,
				TimeoutHours: 24,
			},
		},
	}
	workflow.ExecuteChildWorkflow(nginxCtx, DependentServiceWorkflow, nginxDependency)

	// Wait for nginx to be ready
	nginxReadySignal := ReadySignalSvc(input.NginxService)
	nginxReadyChan := workflow.GetSignalChannel(ctx, nginxReadySignal)
	var nginxReady bool
	nginxReadyChan.Receive(ctx, &nginxReady)
	logger.Info("Nginx ready")

	// Phase 4: Start application services (depend on nginx)
	for _, svcName := range input.AfterNginxServices {
		logger.Info("Starting application service", "service", svcName)

		svcConfig := NodeWorkflowConfig{
			Name:             svcName,
			SystemdUnit:      fmt.Sprintf("%s.service", svcName),
			ServiceName:      svcName,
			CheckSync:        false,
			ReadySignal:      ReadySignalSvc(svcName),
			ParentWorkflowID: WorkflowIDInfra(),
			WatchInterval:    30 * time.Second,
			MaxRestarts:      5,
			RestartBackoff:   10 * time.Second,
		}

		svcWorkflowID := WorkflowIDSvc(svcName)
		svcCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID: svcWorkflowID,
		})

		svcDependency := DependentServiceConfig{
			NodeConfig: svcConfig,
			Dependencies: []DependencyInfo{
				{
					WorkflowID:   WorkflowIDInfra(),
					SignalNames:  []string{nginxReadySignal},
					RequiredAny:  false,
					TimeoutHours: 24,
				},
			},
		}
		workflow.ExecuteChildWorkflow(svcCtx, DependentServiceWorkflow, svcDependency)
	}

	logger.Info("All infrastructure components started and orchestrated")

	// Keep running and monitoring
	workflow.GetSignalChannel(ctx, "Shutdown").Receive(ctx, nil)
	return nil
}
