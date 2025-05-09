package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/pierreaubert/dotidx/dix"
)

func main() {
	log.Println("Starting Dix Watcher...")

	// Load service tree configuration from dix.GetServiceTree (defined in dix/mgrconfig.go or similar)
	// The types like dix.ServiceNode are now directly part of the 'dix' package
	serviceTree, err := dix.GetServiceTree()
	if err != nil {
		log.Fatalf("Failed to load service tree configuration: %v", err)
	}
	if len(serviceTree) == 0 {
		log.Println("Warning: Service tree is empty. Watcher will not manage any services.")
		// Consider if the application should exit or run idly if no services are defined.
		// For now, it will continue and run idly if the tree is empty.
	}

	// Load manager operational configuration from dix.GetManagerConfig
	// The type dix.ManagerConfig is also now directly part of the 'dix' package
	managerConfig, err := dix.GetManagerConfig()
	if err != nil {
		log.Fatalf("Failed to load manager configuration: %v", err)
	}

	// Create the service manager (NewServiceManager is now in the 'dix' package)
	manager, err := dix.NewServiceManager(serviceTree, managerConfig)
	if err != nil {
		log.Fatalf("Failed to create service manager: %v", err)
	}

	// Use a context for graceful shutdown of the manager and its watchers
	ctx, stopManager := context.WithCancel(context.Background())

	// Handle OS signals (SIGINT, SIGTERM) for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v. Initiating shutdown...", sig)
		stopManager() // Cancel the main context, which will propagate to watchers
	}()

	log.Println("Service manager starting to watch services...")
	manager.StartTree(ctx) // Start managing the tree; this will launch watcher goroutines

	// Keep the main goroutine alive until the context is cancelled (e.g., by an OS signal)
	<-ctx.Done()

	log.Println("Shutdown signal processed. Stopping service manager and watchers...")
	manager.StopTree() // This will stop watchers and wait for them to complete
	log.Println("Dix Watcher stopped gracefully. Exiting application.")
}
