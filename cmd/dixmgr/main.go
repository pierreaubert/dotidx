package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
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

	// Medium-priority feature flags
	enableCircuitBreaker := flag.Bool("circuit-breaker", true, "Enable circuit breaker pattern")
	enableHealthHistory := flag.Bool("health-history", false, "Enable persistent health history")
	healthHistoryDB := flag.String("health-history-db", "/var/lib/dixmgr/health.db", "Health history database path")
	enableDynamicConfig := flag.Bool("dynamic-config", true, "Enable dynamic configuration")
	configPort := flag.Int("config-port", 9091, "Configuration API port")

	// Process manager flags
	processManagerType := flag.String("process-manager", "systemd", "Process manager type: systemd or direct")
	processLogDir := flag.String("process-log-dir", "/var/log/dixmgr", "Directory for process logs (direct mode)")
	processPIDDir := flag.String("process-pid-dir", "/var/run/dixmgr", "Directory for PID files (direct mode)")
	processMaxRestarts := flag.Int("process-max-restarts", 5, "Maximum restart attempts per process")

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
	log.Printf("High-priority features: metrics=%v, alerts=%v, resource-monitoring=%v",
		*metricsEnabled, *alertsEnabled, *enableResourceMonitoring)
	log.Printf("Medium-priority features: circuit-breaker=%v, health-history=%v, dynamic-config=%v",
		*enableCircuitBreaker, *enableHealthHistory, *enableDynamicConfig)
	log.Printf("Process manager: type=%s, log-dir=%s, pid-dir=%s, max-restarts=%d",
		*processManagerType, *processLogDir, *processPIDDir, *processMaxRestarts)

	// Load configuration
	config, err := dix.LoadMgrConfig(*configFile)
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Initialize metrics collector
	var metricsCollector *MetricsCollector
	if *metricsEnabled {
		metricsCollector = NewMetricsCollector("dixmgr")
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

	// Initialize circuit breaker manager
	var circuitBreakerManager *CircuitBreakerManager
	if *enableCircuitBreaker {
		cbConfig := CircuitBreakerConfig{
			MaxFailures:      5,
			Timeout:          60 * time.Second,
			HalfOpenRequests: 3,
		}
		circuitBreakerManager = NewCircuitBreakerManager(cbConfig, metricsCollector)
		log.Printf("Circuit breaker manager initialized")
	}

	// Initialize health history store
	var healthHistory *HealthHistoryStore
	if *enableHealthHistory {
		healthHistory, err = NewHealthHistoryStore(*healthHistoryDB, true)
		if err != nil {
			log.Fatalf("Failed to initialize health history store: %v", err)
		}
		defer healthHistory.Close()
		log.Printf("Health history store initialized: %s", *healthHistoryDB)

		// Start background purge task (keep 30 days of data)
		go func() {
			ticker := time.NewTicker(24 * time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				if err := healthHistory.PurgeOldData(30 * 24 * time.Hour); err != nil {
					log.Printf("Warning: failed to purge old health data: %v", err)
				}
			}
		}()
	}

	// Initialize dynamic configuration
	var dynamicConfig *DynamicConfig
	if *enableDynamicConfig {
		dynamicConfig = NewDynamicConfig()
		dynamicConfig.MetricsEnabled = *metricsEnabled
		dynamicConfig.AlertsEnabled = *alertsEnabled
		dynamicConfig.ResourceMonitoringEnabled = *enableResourceMonitoring
		dynamicConfig.HealthHistoryEnabled = *enableHealthHistory
		dynamicConfig.MetricsPort = *metricsPort

		// Start config HTTP server
		configServer := NewConfigHTTPServer(dynamicConfig)
		configServer.RegisterHandlers()
		go func() {
			addr := fmt.Sprintf(":%d", *configPort)
			log.Printf("Starting configuration API server on %s", addr)
			if err := http.ListenAndServe(addr, nil); err != nil {
				log.Printf("Configuration server error: %v", err)
			}
		}()
		log.Printf("Dynamic configuration enabled (API on port %d)", *configPort)
	}

	// Initialize process manager
	var processManager ProcessManager
	pmConfig := ProcessManagerConfig{
		Type:         ProcessManagerType(*processManagerType),
		LogDir:       *processLogDir,
		PIDDir:       *processPIDDir,
		MaxRestarts:  *processMaxRestarts,
		UseCgroups:   false, // Can be made configurable if needed
	}

	processManager, err = NewProcessManager(pmConfig, metricsCollector)
	if err != nil {
		log.Fatalf("Failed to create process manager: %v", err)
	}
	defer processManager.Close()
	log.Printf("Process manager initialized: type=%s", processManager.Name())

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

	// Create activities instance with all features
	activities, err := NewActivities(*execMode, metricsCollector, alertManager, *enableResourceMonitoring, circuitBreakerManager, healthHistory, dynamicConfig, processManager)
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

	// Register process management activities
	w.RegisterActivity(activities.StartProcessActivity)
	w.RegisterActivity(activities.StopProcessActivity)
	w.RegisterActivity(activities.RestartProcessActivity)
	w.RegisterActivity(activities.CheckProcessActivity)
	w.RegisterActivity(activities.GetProcessOutputActivity)
	w.RegisterActivity(activities.KillProcessActivity)
	w.RegisterActivity(activities.ListProcessesActivity)

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
