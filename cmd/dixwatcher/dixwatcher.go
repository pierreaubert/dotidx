package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/pierreaubert/dotidx/dix"
)

func GetServiceTree(config dix.MgrConfig) (sm []*dix.ServiceNode, err error) {

	databaseNode := &dix.ServiceNode{
		Name:        "DixDatabase",
		SystemdUnit: "postgres@16-dotidx.service",
		IsLeaf:      true,
		Children:    nil,
	}

	parachainParentNode := &dix.ServiceNode{
		Name:     "Parachains",
		IsLeaf:   false,
		Children: []*dix.ServiceNode{},
	}

	for relayChain, chainConfig := range config.Parachains {
		relay := &dix.ServiceNode{
			Name:        fmt.Sprintf("RelayChain-%s", relayChain),
			SystemdUnit: fmt.Sprintf("relay-node-archive@%s.service", relayChain),
			IsLeaf:      true,
			Children:    nil,
		}
		for chain, parachainCfg := range chainConfig {
			if relayChain == chain {
				continue
			}
			node := &dix.ServiceNode{
				Name:        fmt.Sprintf("Chain-%s-%s", relayChain, chain),
				SystemdUnit: fmt.Sprintf("chain-node-archive@%s-%s.service", relayChain, chain),
				IsLeaf:      false,
				Children:    []*dix.ServiceNode{relay},
			}

			// loop and create N sidecar
			for i := range parachainCfg.SidecarCount {
				sidecar := &dix.ServiceNode{
					Name:        fmt.Sprintf("Sidecar-%s-%s-%d", relayChain, chain, i),
					SystemdUnit: fmt.Sprintf("sidecar@%s-%s-%d.service", relayChain, chain, i),
					IsLeaf:      false,
					Children:    []*dix.ServiceNode{node},
				}
				parachainParentNode.Children = append(parachainParentNode.Children, sidecar)
			}
		}
	}

	nginxNode := &dix.ServiceNode{
		Name:        "DixNginx",
		SystemdUnit: "dix-nginx.service",
		IsLeaf:      false,
		Children:    parachainParentNode.Children,
	}

	frontendNode := &dix.ServiceNode{
		Name:        "DixFrontend",
		SystemdUnit: "dixfe.service",
		IsLeaf:      false,
		Children:    []*dix.ServiceNode{nginxNode, databaseNode},
	}

	liveNode := &dix.ServiceNode{
		Name:        "DixLive",
		SystemdUnit: "dixlive.service",
		IsLeaf:      false,
		Children:    []*dix.ServiceNode{nginxNode, databaseNode},
	}

	cronNode := &dix.ServiceNode{
		Name:        "DixCron",
		SystemdUnit: "dixcron.service",
		IsLeaf:      false,
		Children:    []*dix.ServiceNode{nginxNode, databaseNode},
	}

	sm = append(sm, frontendNode)
	sm = append(sm, liveNode)
	sm = append(sm, cronNode)

	return
}

func main() {

	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	configFile := flag.String("conf", "", "toml configuration file")
	flag.Parse()

	log.Printf("Starting Dix Watcher with configuration file : %s.", *configFile)

	config, err := dix.LoadMgrConfig(*configFile)
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	serviceTree, err := GetServiceTree(*config)
	if err != nil {
		log.Fatalf("Failed to load service tree configuration: %v", err)
	}
	if len(serviceTree) == 0 {
		log.Fatalf("Serviice tree is empty. Watcher will not manage any services.")
	}

	manager, err := dix.NewServiceManager(serviceTree, (*config).Watcher)
	if err != nil {
		log.Fatalf("Failed to create service manager: %v", err)
	}

	ctx, stopManager := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v. Initiating shutdown...", sig)
		stopManager()
	}()

	log.Println("Service manager starting to watch services...")
	manager.StartTree(ctx) // Start managing the tree; this will launch watcher goroutines

	// Keep the main goroutine alive until the context is cancelled (e.g., by an OS signal)
	<-ctx.Done()

	log.Println("Shutdown signal processed. Stopping service manager and watchers...")
	manager.StopTree() // This will stop watchers and wait for them to complete
	log.Println("Dix Watcher stopped gracefully. Exiting application.")
}
