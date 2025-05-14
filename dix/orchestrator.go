package dix

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
)

// dbusConnInterface defines the subset of dbus.Conn methods used by ServiceManager.
// This allows for easier mocking in tests.
type dbusConnInterface interface {
	Close()
	GetUnitProperties(unit string) (map[string]interface{}, error)
	StartUnit(name string, mode string, ch chan<- string) (int, error)
	StopUnit(name string, mode string, ch chan<- string) (int, error)
	RestartUnit(name string, mode string, ch chan<- string) (int, error)
	// Add other dbus.Conn methods here if ServiceManager starts using them.
}

var newDbusConnectionFunc = func() (dbusConnInterface, error) { return dbus.New() } // Allow overriding for tests

// ServiceHealthStatus represents the health of a managed service
type ServiceHealthStatus string

const (
	StatusUnknown ServiceHealthStatus = "UNKNOWN"
	StatusOk      ServiceHealthStatus = "OK"
	StatusKo      ServiceHealthStatus = "KO" // "Knocked Out" / Not OK
)

// ServiceNode represents a node in the service tree
type ServiceNode struct {
	Name        string // Logical name for this node
	SystemdUnit string // Actual systemd unit name (if this node is a manageable service)
	IsLeaf      bool   // True if this is a leaf node that should be directly managed/watched
	Children    []*ServiceNode

	// Runtime state for managed leaf services
	mu            sync.Mutex
	currentStatus ServiceHealthStatus
	lastCheckTime time.Time
	restartCount  int
	cancelWatcher context.CancelFunc // To stop the watcher goroutine for this service
}

type ServiceManager struct {
	RootNodes []*ServiceNode
	dbusConn  dbusConnInterface
	config    OrchestratorConfig
	wg        sync.WaitGroup // To wait for all watchers to complete on shutdown
}

func NewServiceManager(rootNodes []*ServiceNode, config OrchestratorConfig) (*ServiceManager, error) {
	conn, err := newDbusConnectionFunc() // Returns dbusConnInterface
	if err != nil {
		return nil, fmt.Errorf("failed to connect to D-Bus: %w", err)
	}
	if config.OperationTimeout == 0 {
		config.OperationTimeout = 30
	}
	return &ServiceManager{
		RootNodes: rootNodes,
		dbusConn:  conn,
		config:    config,
	}, nil
}

// StartTree initiates the management of the service tree.
func (sm *ServiceManager) StartTree(ctx context.Context) {
	log.Println("Service Manager: Starting service tree...")
	for _, rootNode := range sm.RootNodes {
		sm.processNodeRecursive(ctx, rootNode, sm.startAndWatchLeaf)
	}
	log.Println("Service Manager: All initial leaf services processed for starting and watching.")
}

// StopTree gracefully stops all watchers and closes the D-Bus connection.
func (sm *ServiceManager) StopTree() {
	log.Println("Service Manager: Stopping service tree...")
	for _, rootNode := range sm.RootNodes {
		sm.processNodeRecursive(context.Background(), rootNode, func(ctx context.Context, node *ServiceNode) {
			node.mu.Lock()
			if node.cancelWatcher != nil {
				log.Printf("Service Manager: Stopping watcher for %s (Unit: %s)", node.Name, node.SystemdUnit)
				node.cancelWatcher()
			}
			node.mu.Unlock()
		})
	}
	sm.wg.Wait() // Wait for all watcher goroutines to finish
	if sm.dbusConn != nil {
		sm.dbusConn.Close()
	}
	log.Println("Service Manager: Shutdown complete.")
}

// processNodeRecursive is a generic tree traversal function.
func (sm *ServiceManager) processNodeRecursive(ctx context.Context, node *ServiceNode, leafAction func(context.Context, *ServiceNode)) {
	if node == nil {
		return
	}
	if node.IsLeaf && node.SystemdUnit != "" {
		leafAction(ctx, node)
	}
	for _, child := range node.Children {
		sm.processNodeRecursive(ctx, child, leafAction)
	}
}

// startAndWatchLeaf is called for each leaf node to start the service and its watcher.
func (sm *ServiceManager) startAndWatchLeaf(ctx context.Context, serviceNode *ServiceNode) {
	serviceNode.mu.Lock()
	if serviceNode.cancelWatcher != nil {
		log.Printf("Watcher for %s (Unit: %s) already running or not cleaned up. Attempting to cancel previous.", serviceNode.Name, serviceNode.SystemdUnit)
		serviceNode.cancelWatcher()
	}

	watcherCtx, cancel := context.WithCancel(ctx) // New context for this watcher
	serviceNode.cancelWatcher = cancel
	serviceNode.currentStatus = StatusUnknown
	serviceNode.restartCount = 0
	serviceNode.mu.Unlock()

	sm.wg.Add(1) // Increment for the new watcher goroutine
	go sm.watchService(watcherCtx, serviceNode)
}

// watchService is the goroutine that monitors a single service.
func (sm *ServiceManager) watchService(ctx context.Context, serviceNode *ServiceNode) {
	defer sm.wg.Done() // Decrement when watcher exits
	defer func() {     // Cleanup cancel function
		serviceNode.mu.Lock()
		serviceNode.cancelWatcher = nil
		serviceNode.mu.Unlock()
		log.Printf("Watcher for %s (Unit: %s) has stopped.", serviceNode.Name, serviceNode.SystemdUnit)
	}()

	log.Printf("Watcher starting for %s (Unit: %s)", serviceNode.Name, serviceNode.SystemdUnit)

	// Initial attempt to ensure the service is active
	isActive, err := sm.checkAndEnsureServiceActive(ctx, serviceNode.SystemdUnit)
	serviceNode.mu.Lock()
	if err != nil {
		log.Printf("Watcher [%s]: Initial start/check failed: %v", serviceNode.SystemdUnit, err)
		serviceNode.currentStatus = StatusKo
	} else if isActive {
		serviceNode.currentStatus = StatusOk
	} else {
		serviceNode.currentStatus = StatusKo
	}
	serviceNode.lastCheckTime = time.Now()
	serviceNode.mu.Unlock()

	ticker := time.NewTicker(sm.config.WatchInterval * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done(): // Context cancelled (e.g., by StopTree or parent context)
			return
		case <-ticker.C:
			props, err := sm.dbusConn.GetUnitProperties(serviceNode.SystemdUnit)
			if err != nil {
				log.Printf("Watcher [%s]: Error getting properties: %v", serviceNode.SystemdUnit, err)
				serviceNode.mu.Lock()
				serviceNode.currentStatus = StatusKo // Treat error getting props as KO
				serviceNode.lastCheckTime = time.Now()
				serviceNode.mu.Unlock()
			} else {
				activeStateVal, okActive := props["ActiveState"]

				serviceNode.mu.Lock()
				serviceNode.lastCheckTime = time.Now()
				if okActive {
					activeState, _ := activeStateVal.(string)
					if activeState == "active" {
						if serviceNode.currentStatus != StatusOk {
							log.Printf("Watcher [%s]: Service is now OK (ActiveState: %s)", serviceNode.SystemdUnit, activeState)
						}
						serviceNode.currentStatus = StatusOk
						serviceNode.restartCount = 0 // Reset restart count on healthy state
					} else {
						if serviceNode.currentStatus != StatusKo {
							log.Printf("Watcher [%s]: Service is KO (ActiveState: %s)", serviceNode.SystemdUnit, activeState)
						}
						serviceNode.currentStatus = StatusKo
					}
				} else {
					log.Printf("Watcher [%s]: ActiveState not found in properties.", serviceNode.SystemdUnit)
					serviceNode.currentStatus = StatusKo // Property missing, assume KO
				}
				serviceNode.mu.Unlock()
			}

			// Check if restart is needed
			serviceNode.mu.Lock()
			if serviceNode.currentStatus == StatusKo && serviceNode.restartCount < sm.config.MaxRestarts {
				serviceNode.restartCount++
				currentRestartAttempt := serviceNode.restartCount
				serviceNode.mu.Unlock()

				if currentRestartAttempt > 1 && sm.config.RestartBackoff > 0 {
					backoffDuration := time.Duration(currentRestartAttempt-1) * sm.config.RestartBackoff
					log.Printf("Watcher [%s]: Applying backoff of %v before restart attempt #%d", serviceNode.SystemdUnit, backoffDuration, currentRestartAttempt)
					select {
					case <-time.After(backoffDuration):
					case <-ctx.Done():
						return
					}
				}

				log.Printf("Watcher [%s]: Attempting restart #%d", serviceNode.SystemdUnit, currentRestartAttempt)
				err := sm.restartSystemdUnit(ctx, serviceNode.SystemdUnit)
				if err != nil {
					log.Printf("Watcher [%s]: Restart attempt #%d failed: %v", serviceNode.SystemdUnit, currentRestartAttempt, err)
				} else {
					log.Printf("Watcher [%s]: Restart attempt #%d initiated successfully.", serviceNode.SystemdUnit, currentRestartAttempt)
				}
			} else if serviceNode.currentStatus == StatusKo && serviceNode.restartCount >= sm.config.MaxRestarts {
				log.Printf("Watcher [%s]: Service is KO and has reached max restart attempts (%d). Not restarting.", serviceNode.SystemdUnit, sm.config.MaxRestarts)
				serviceNode.mu.Unlock()
			} else {
				serviceNode.mu.Unlock()
			}
		}
	}
}

// systemdOperation abstracts common systemd D-Bus call patterns.
func (sm *ServiceManager) systemdOperation(ctx context.Context, unitName string, operation func(string, string, chan<- string) (int, error)) error {
	opChan := make(chan string)
	jobID, err := operation(unitName, "replace", opChan) // "replace" is a common mode
	if err != nil {
		return fmt.Errorf("failed to schedule systemd job for %s: %w", unitName, err)
	}
	if jobID == 0 {
		log.Printf("Systemd operation for %s started (no job ID returned, assuming immediate or already in target state).", unitName)
		return nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, sm.config.OperationTimeout)
	defer cancel()

	select {
	case result := <-opChan:
		if result == "done" {
			log.Printf("Systemd operation for %s completed: %s", unitName, result)
			return nil
		}
		return fmt.Errorf("systemd operation for %s finished with result: %s", unitName, result)
	case <-timeoutCtx.Done():
		return fmt.Errorf("timeout waiting for systemd operation on %s: %w", unitName, timeoutCtx.Err())
	}
}

func (sm *ServiceManager) startSystemdUnit(ctx context.Context, unitName string) error {
	log.Printf("Systemd: Starting unit %s", unitName)
	return sm.systemdOperation(ctx, unitName, sm.dbusConn.StartUnit)
}

func (sm *ServiceManager) stopSystemdUnit(ctx context.Context, unitName string) error {
	log.Printf("Systemd: Stopping unit %s", unitName)
	return sm.systemdOperation(ctx, unitName, sm.dbusConn.StopUnit)
}

func (sm *ServiceManager) restartSystemdUnit(ctx context.Context, unitName string) error {
	log.Printf("Systemd: Restarting unit %s", unitName)
	return sm.systemdOperation(ctx, unitName, sm.dbusConn.RestartUnit)
}

// checkAndEnsureServiceActive checks if a service is active, and tries to start it if not.
// Returns true if active (or successfully started), false otherwise.
func (sm *ServiceManager) checkAndEnsureServiceActive(ctx context.Context, unitName string) (bool, error) {
	props, err := sm.dbusConn.GetUnitProperties(unitName)
	if err != nil {
		return false, fmt.Errorf("failed to get properties for %s: %w", unitName, err)
	}

	activeState, ok := props["ActiveState"].(string)
	if !ok {
		return false, fmt.Errorf("ActiveState for %s is not a string or not found", unitName)
	}

	if activeState == "active" {
		return true, nil
	}

	log.Printf("Service %s is not active (state: %s). Attempting to start.", unitName, activeState)
	err = sm.startSystemdUnit(ctx, unitName)
	if err != nil {
		return false, fmt.Errorf("failed to start service %s: %w", unitName, err)
	}

	// After starting, re-check properties to confirm it became active
	// The startSystemdUnit function already waits for the operation to complete or timeout.
	// If ctx was cancelled during startSystemdUnit, it would have returned an error.
	// If ctx is cancelled after startSystemdUnit but before GetUnitProperties, we should still try to get properties if possible,
	// or let the GetUnitProperties call handle the context cancellation if it's designed to.
	// However, if the primary context for the watcher is cancelled, GetUnitProperties might fail or return stale data.
	// For now, let's assume if startSystemdUnit succeeded, we can proceed to check state.

	// If the context is already done, we might not want to proceed or expect a specific error.
	// Checking ctx.Err() here can prevent further operations if the context is already cancelled.
	if ctx.Err() != nil {
		log.Printf("Context cancelled before re-checking properties for %s: %v", unitName, ctx.Err())
		return false, ctx.Err()
	}

	updatedProps, err := sm.dbusConn.GetUnitProperties(unitName)
	if err != nil {
		return false, fmt.Errorf("failed to get properties for %s after start attempt: %w", unitName, err)
	}
	updatedActiveState, ok := updatedProps["ActiveState"].(string)
	if !ok {
		return false, fmt.Errorf("ActiveState for %s after start attempt is not a string or not found", unitName)
	}

	if updatedActiveState != "active" {
		log.Printf("Service %s still not active (state: %s) after start attempt.", unitName, updatedActiveState)
	}

	return updatedActiveState == "active", nil
}
