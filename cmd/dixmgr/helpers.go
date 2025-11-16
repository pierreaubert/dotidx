package main

import (
	"fmt"

	"github.com/pierreaubert/dotidx/dix"
)

// Signal naming helpers
func ReadySignalRelay(relay string) string {
	return fmt.Sprintf("ready:relay:%s", relay)
}

func ReadySignalPara(relay, chain string) string {
	return fmt.Sprintf("ready:para:%s:%s", relay, chain)
}

func ReadySignalSidecar(relay, chain string, idx int) string {
	return fmt.Sprintf("ready:sidecar:%s:%s:%d", relay, chain, idx)
}

func ReadySignalSvc(name string) string {
	return fmt.Sprintf("ready:svc:%s", name)
}

// Workflow ID helpers
func WorkflowIDInfra() string {
	return "wf.infra"
}

func WorkflowIDNodeRelay(relay string) string {
	return fmt.Sprintf("wf.node.relay.%s", relay)
}

func WorkflowIDNodePara(relay, chain string) string {
	return fmt.Sprintf("wf.node.para.%s.%s", relay, chain)
}

func WorkflowIDSidecar(relay, chain string, idx int) string {
	return fmt.Sprintf("wf.sidecar.%s.%s.%d", relay, chain, idx)
}

func WorkflowIDSvc(name string) string {
	return fmt.Sprintf("wf.svc.%s", name)
}

func WorkflowIDBatch(relay, chain string) string {
	return fmt.Sprintf("wf.batch.%s.%s", relay, chain)
}

func WorkflowIDCron(schedule string) string {
	return fmt.Sprintf("wf.cron.%s", schedule)
}

// FromMgrConfigToInfraInput converts MgrConfig to InfrastructureWorkflowInput
// It validates port conventions and derives service names, RPC ports, and signals
func FromMgrConfigToInfraInput(cfg *dix.MgrConfig, watchInterval, maxRestarts int, restartBackoff int) (InfrastructureWorkflowInput, error) {
	input := InfrastructureWorkflowInput{
		NginxService:       "dix-nginx",
		AfterNginxServices: []string{"dixlive", "dixfe", "dixbatch", "dixcron"},
	}

	// Process each relay chain
	for relayName, chainConfigs := range cfg.Parachains {
		relayPlan := RelayPlan{
			RelayID: relayName,
		}

		// Find relay chain config (relay chain has same name as its key)
		if relayConfig, exists := chainConfigs[relayName]; exists {
			relayPlan.Node = NodeWorkflowConfig{
				Name:             fmt.Sprintf("RelayChain-%s", relayName),
				SystemdUnit:      fmt.Sprintf("relay-node-archive@%s.service", relayName),
				ServiceName:      relayName,
				RPCPort:          relayConfig.PortRPC,
				CheckSync:        true,
				ReadySignal:      ReadySignalRelay(relayName),
				ParentWorkflowID: WorkflowIDInfra(),
			}
		}

		// Process parachains attached to this relay
		for chainName, chainConfig := range chainConfigs {
			// Skip if it's the relay chain itself
			if chainName == relayName {
				continue
			}

			paraPlan := ParaPlan{
				ChainID:            chainName,
				SidecarServiceName: fmt.Sprintf("sidecar-%s-%s", relayName, chainName),
				SidecarCount:       chainConfig.SidecarCount,
			}

			// Parachain node configuration
			paraPlan.Node = NodeWorkflowConfig{
				Name:             fmt.Sprintf("Chain-%s-%s", relayName, chainName),
				SystemdUnit:      fmt.Sprintf("chain-node-archive@%s-%s.service", relayName, chainName),
				ServiceName:      fmt.Sprintf("%s-%s", relayName, chainName),
				RPCPort:          chainConfig.PortRPC,
				CheckSync:        true,
				ReadySignal:      ReadySignalPara(relayName, chainName),
				ParentWorkflowID: WorkflowIDInfra(),
			}

			relayPlan.Parachains = append(relayPlan.Parachains, paraPlan)
		}

		input.RelayPlans = append(input.RelayPlans, relayPlan)
	}

	return input, nil
}
