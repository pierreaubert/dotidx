package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pierreaubert/dotidx/dix"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	configFile := flag.String("conf", "", "toml configuration file")
	temporalHost := flag.String("temporal-host", "localhost:7233", "Temporal server address")
	temporalNamespace := flag.String("temporal-namespace", "default", "Temporal namespace")
	flag.Parse()

	if *configFile == "" {
		log.Fatal("Configuration file is required (use -conf flag)")
	}

	log.Printf("Starting Dix Watcher with configuration file: %s", *configFile)
	log.Printf("Temporal server: %s, namespace: %s", *temporalHost, *temporalNamespace)

	// Load configuration
	config, err := dix.LoadMgrConfig(*configFile)
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create Temporal client
	temporalClient, err := client.Dial(client.Options{
		HostPort:  *temporalHost,
		Namespace: *temporalNamespace,
	})
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer temporalClient.Close()

	log.Println("Connected to Temporal server")

	// Create activities instance
	activities, err := NewActivities()
	if err != nil {
		log.Fatalf("Failed to create activities: %v", err)
	}
	defer activities.Close()

	// Create and start worker
	taskQueue := "dotidx-watcher"
	w := worker.New(temporalClient, taskQueue, worker.Options{})

	// Register workflows
	w.RegisterWorkflow(NodeWorkflow)

	// Register activities
	w.RegisterActivity(activities.CheckSystemdServiceActivity)
	w.RegisterActivity(activities.StartSystemdServiceActivity)
	w.RegisterActivity(activities.StopSystemdServiceActivity)
	w.RegisterActivity(activities.RestartSystemdServiceActivity)

	log.Printf("Registered workflows and activities on task queue: %s", taskQueue)

	// Start worker in background
	err = w.Start()
	if err != nil {
		log.Fatalf("Failed to start worker: %v", err)
	}
	defer w.Stop()

	log.Println("Worker started successfully")

	// Start workflows for each service in the config
	err = startServiceWorkflows(temporalClient, *config, taskQueue)
	if err != nil {
		log.Fatalf("Failed to start service workflows: %v", err)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Dix Watcher is running. Press Ctrl+C to stop.")
	sig := <-sigChan
	log.Printf("Received signal %v. Initiating shutdown...", sig)

	log.Println("Dix Watcher stopped gracefully. Exiting application.")
}

// startServiceWorkflows creates and starts Temporal workflows for all configured services
func startServiceWorkflows(c client.Client, config dix.MgrConfig, taskQueue string) error {
	ctx := context.Background()

	// Get default watch settings from config
	watchInterval := config.Watcher.WatchInterval
	if watchInterval == 0 {
		watchInterval = 30 * time.Second
	}

	maxRestarts := config.Watcher.MaxRestarts
	if maxRestarts == 0 {
		maxRestarts = 5
	}

	restartBackoff := config.Watcher.RestartBackoff
	if restartBackoff == 0 {
		restartBackoff = 10 * time.Second
	}

	log.Printf("Default settings: watchInterval=%v, maxRestarts=%d, restartBackoff=%v",
		watchInterval, maxRestarts, restartBackoff)

	// Start PostgreSQL workflow
	postgresConfig := NodeWorkflowConfig{
		Name:           "PostgreSQL",
		SystemdUnit:    "postgres@16-dotidx.service",
		WatchInterval:  watchInterval,
		MaxRestarts:    maxRestarts,
		RestartBackoff: restartBackoff,
	}

	err := startNodeWorkflow(ctx, c, postgresConfig, taskQueue)
	if err != nil {
		return fmt.Errorf("failed to start PostgreSQL workflow: %w", err)
	}

	// Start relay chain and parachain workflows
	for relayChain, chainConfigs := range config.Parachains {
		// Start relay chain node
		relayConfig := NodeWorkflowConfig{
			Name:           fmt.Sprintf("RelayChain-%s", relayChain),
			SystemdUnit:    fmt.Sprintf("relay-node-archive@%s.service", relayChain),
			WatchInterval:  watchInterval,
			MaxRestarts:    maxRestarts,
			RestartBackoff: restartBackoff,
		}

		err := startNodeWorkflow(ctx, c, relayConfig, taskQueue)
		if err != nil {
			return fmt.Errorf("failed to start relay chain workflow for %s: %w", relayChain, err)
		}

		// Start parachain nodes and sidecars
		for chain, parachainCfg := range chainConfigs {
			if relayChain == chain {
				continue // Skip if it's the relay chain itself
			}

			// Start parachain node
			parachainConfig := NodeWorkflowConfig{
				Name:           fmt.Sprintf("Chain-%s-%s", relayChain, chain),
				SystemdUnit:    fmt.Sprintf("chain-node-archive@%s-%s.service", relayChain, chain),
				WatchInterval:  watchInterval,
				MaxRestarts:    maxRestarts,
				RestartBackoff: restartBackoff,
			}

			err := startNodeWorkflow(ctx, c, parachainConfig, taskQueue)
			if err != nil {
				return fmt.Errorf("failed to start parachain workflow for %s-%s: %w", relayChain, chain, err)
			}

			// Start sidecar instances
			for i := 0; i < parachainCfg.SidecarCount; i++ {
				sidecarConfig := NodeWorkflowConfig{
					Name:           fmt.Sprintf("Sidecar-%s-%s-%d", relayChain, chain, i),
					SystemdUnit:    fmt.Sprintf("sidecar@%s-%s-%d.service", relayChain, chain, i),
					WatchInterval:  watchInterval,
					MaxRestarts:    maxRestarts,
					RestartBackoff: restartBackoff,
				}

				err := startNodeWorkflow(ctx, c, sidecarConfig, taskQueue)
				if err != nil {
					return fmt.Errorf("failed to start sidecar workflow for %s-%s-%d: %w", relayChain, chain, i, err)
				}
			}
		}
	}

	// Start nginx workflow
	nginxConfig := NodeWorkflowConfig{
		Name:           "Nginx",
		SystemdUnit:    "dix-nginx.service",
		WatchInterval:  watchInterval,
		MaxRestarts:    maxRestarts,
		RestartBackoff: restartBackoff,
	}

	err = startNodeWorkflow(ctx, c, nginxConfig, taskQueue)
	if err != nil {
		return fmt.Errorf("failed to start nginx workflow: %w", err)
	}

	// Start dixfe workflow
	dixfeConfig := NodeWorkflowConfig{
		Name:           "DixFE",
		SystemdUnit:    "dixfe.service",
		WatchInterval:  watchInterval,
		MaxRestarts:    maxRestarts,
		RestartBackoff: restartBackoff,
	}

	err = startNodeWorkflow(ctx, c, dixfeConfig, taskQueue)
	if err != nil {
		return fmt.Errorf("failed to start dixfe workflow: %w", err)
	}

	// Start dixlive workflow
	dixliveConfig := NodeWorkflowConfig{
		Name:           "DixLive",
		SystemdUnit:    "dixlive.service",
		WatchInterval:  watchInterval,
		MaxRestarts:    maxRestarts,
		RestartBackoff: restartBackoff,
	}

	err = startNodeWorkflow(ctx, c, dixliveConfig, taskQueue)
	if err != nil {
		return fmt.Errorf("failed to start dixlive workflow: %w", err)
	}

	// Start dixcron workflow
	dixcronConfig := NodeWorkflowConfig{
		Name:           "DixCron",
		SystemdUnit:    "dixcron.service",
		WatchInterval:  watchInterval,
		MaxRestarts:    maxRestarts,
		RestartBackoff: restartBackoff,
	}

	err = startNodeWorkflow(ctx, c, dixcronConfig, taskQueue)
	if err != nil {
		return fmt.Errorf("failed to start dixcron workflow: %w", err)
	}

	log.Println("All service workflows started successfully")
	return nil
}

// startNodeWorkflow starts a single NodeWorkflow for a service
func startNodeWorkflow(ctx context.Context, c client.Client, config NodeWorkflowConfig, taskQueue string) error {
	workflowID := fmt.Sprintf("node-%s", config.Name)

	workflowOptions := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: taskQueue,
		// WorkflowExecutionTimeout: 0, // Infinite - runs until cancelled
		// WorkflowRunTimeout: 0, // Infinite
	}

	log.Printf("Starting workflow for service: %s (workflowID: %s)", config.Name, workflowID)

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, NodeWorkflow, config)
	if err != nil {
		return fmt.Errorf("failed to execute workflow for %s: %w", config.Name, err)
	}

	log.Printf("Started workflow for %s: WorkflowID=%s, RunID=%s", config.Name, we.GetID(), we.GetRunID())
	return nil
}
