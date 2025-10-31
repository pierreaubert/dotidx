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
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	configFile := flag.String("conf", "", "toml configuration file")
	temporalHost := flag.String("temporal-host", "localhost:7233", "Temporal server address")
	temporalNamespace := flag.String("temporal-namespace", "dotidx", "Temporal namespace")
	watchMode := flag.Bool("watch", false, "watch mode: monitor services and print what would be done (dry-run)")
	execMode := flag.Bool("exec", false, "exec mode: monitor services and execute restart actions")
	flag.Parse()

	if *configFile == "" {
		log.Fatal("Configuration file is required (use -conf flag)")
	}

	// Validate mode flags
	if *watchMode && *execMode {
		log.Fatal("Cannot use both -watch and -exec flags. Choose one mode.")
	}
	if !*watchMode && !*execMode {
		log.Fatal("Must specify either -watch (dry-run) or -exec (execute actions) mode")
	}

	mode := "watch (dry-run)"
	if *execMode {
		mode = "exec (execute actions)"
	}
	log.Printf("Starting Dix Watcher in %s mode with configuration file: %s", mode, *configFile)
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

	// Create activities instance with execution mode
	activities, err := NewActivities(*execMode)
	if err != nil {
		log.Fatalf("Failed to create activities: %v", err)
	}
	defer activities.Close()

	// Create and start worker
	taskQueue := "dotidx-watcher"
	w := worker.New(temporalClient, taskQueue, worker.Options{})

	// Register workflows
	w.RegisterWorkflow(NodeWorkflow)
	w.RegisterWorkflow(ClusterWorkflowExample)
	w.RegisterWorkflow(InfrastructureWorkflow)
	w.RegisterWorkflow(DependentServiceWorkflow)

	// Register activities
	w.RegisterActivity(activities.CheckSystemdServiceActivity)
	w.RegisterActivity(activities.StartSystemdServiceActivity)
	w.RegisterActivity(activities.StopSystemdServiceActivity)
	w.RegisterActivity(activities.RestartSystemdServiceActivity)
	w.RegisterActivity(activities.CheckNodeSyncActivity)

	log.Printf("Registered workflows and activities on task queue: %s", taskQueue)

	// Start worker in background
	err = w.Start()
	if err != nil {
		log.Fatalf("Failed to start worker: %v", err)
	}
	defer w.Stop()

	log.Println("Worker started successfully")

	// Build infrastructure workflow input from config
	input, err := FromMgrConfigToInfraInput(config, int(config.Watcher.WatchInterval.Seconds()), config.Watcher.MaxRestarts, int(config.Watcher.RestartBackoff.Seconds()))
	if err != nil {
		log.Fatalf("Failed to build infrastructure input: %v", err)
	}

	// Start the single InfrastructureWorkflow
	err = startInfrastructureWorkflow(temporalClient, input, taskQueue)
	if err != nil {
		log.Fatalf("Failed to start infrastructure workflow: %v", err)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Dix Watcher is running. Press Ctrl+C to stop.")
	sig := <-sigChan
	log.Printf("Received signal %v. Initiating shutdown...", sig)

	log.Println("Dix Watcher stopped gracefully. Exiting application.")
}

// startInfrastructureWorkflow starts the root infrastructure orchestration workflow
func startInfrastructureWorkflow(c client.Client, input InfrastructureWorkflowInput, taskQueue string) error {
	ctx := context.Background()

	log.Printf("Starting InfrastructureWorkflow with %d relay chains", len(input.RelayPlans))

	workflowID := WorkflowIDInfra()
	workflowOptions := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: taskQueue,
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, InfrastructureWorkflow, input)
	if err != nil {
		return fmt.Errorf("failed to execute InfrastructureWorkflow: %w", err)
	}

	log.Printf("Started InfrastructureWorkflow: WorkflowID=%s, RunID=%s", we.GetID(), we.GetRunID())
	return nil
}
