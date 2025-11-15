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
	temporalNamespace := flag.String("temporal-namespace", "dotidx", "Temporal namespace")
	watchMode := flag.Bool("watch", false, "watch mode: monitor services and print what would be done (dry-run)")
	execMode := flag.Bool("exec", false, "exec mode: monitor services and execute restart actions")

	// New flags for enhanced features
	metricsEnabled := flag.Bool("metrics", true, "Enable Prometheus metrics")
	metricsPort := flag.Int("metrics-port", 9090, "Metrics server port")
	alertsEnabled := flag.Bool("alerts", true, "Enable alerting")
	slackWebhook := flag.String("slack-webhook", "", "Slack webhook URL for alerts")
	webhookURL := flag.String("webhook-url", "", "Generic webhook URL for alerts")
	enableResourceMonitoring := flag.Bool("resource-monitoring", true, "Enable resource monitoring")

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
	log.Printf("Features: metrics=%v, alerts=%v, resource-monitoring=%v",
		*metricsEnabled, *alertsEnabled, *enableResourceMonitoring)

	// Load configuration
	config, err := dix.LoadMgrConfig(*configFile)
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Initialize metrics collector
	var metricsCollector *MetricsCollector
	if *metricsEnabled {
		metricsCollector = NewMetricsCollector("dixwatcher")
		log.Printf("Metrics collector initialized")

		// Start metrics server in background
		go func() {
			addr := fmt.Sprintf(":%d", *metricsPort)
			log.Printf("Starting metrics server on %s", addr)
			if err := metricsCollector.StartMetricsServer(addr); err != nil {
				log.Printf("Metrics server error: %v", err)
			}
		}()
	}

	// Initialize alert manager
	var alertManager *AlertManager
	if *alertsEnabled {
		alertManager = NewAlertManager(metricsCollector, 5*time.Minute)

		// Register log channel (always enabled)
		alertManager.RegisterChannel(NewLogChannel())

		// Register Slack channel if webhook provided
		if *slackWebhook != "" {
			alertManager.RegisterChannel(NewSlackChannel(*slackWebhook))
			log.Printf("Registered Slack alert channel")
		}

		// Register generic webhook if provided
		if *webhookURL != "" {
			alertManager.RegisterChannel(NewWebhookChannel(*webhookURL, nil))
			log.Printf("Registered webhook alert channel: %s", *webhookURL)
		}

		log.Printf("Alert manager initialized")
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

	// Create activities instance with enhanced features
	activities, err := NewActivities(*execMode, metricsCollector, alertManager, *enableResourceMonitoring)
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
	w.RegisterActivity(activities.CheckResourceUsageActivity)
	w.RegisterActivity(activities.CheckHTTPEndpointActivity)
	w.RegisterActivity(activities.CheckHTTPEndpointSimpleActivity)

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
