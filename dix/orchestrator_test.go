package dix

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

// Compile-time check to ensure mockDbusConnection implements dbusConnInterface
var _ dbusConnInterface = (*mockDbusConnection)(nil)

func TestNewServiceManager(t *testing.T) {
	originalNewDbusConnFunc := newDbusConnectionFunc
	defer func() { newDbusConnectionFunc = originalNewDbusConnFunc }()

	t.Run("SuccessfulCreation", func(t *testing.T) {
		newDbusConnectionFunc = func() (dbusConnInterface, error) {
			return newMockDbusConnection(), nil // Use the constructor for our mock
		}

		cfg := ManagerConfig{
			WatchInterval:  5 * time.Second,
			MaxRestarts:    3,
			RestartBackoff: 1 * time.Second,
			// OperationTimeout is deliberately omitted to test default
		}
		sm, err := NewServiceManager(nil, cfg)
		if err != nil {
			t.Fatalf("NewServiceManager() error = %v, wantErr %v", err, false)
		}
		if sm == nil {
			t.Fatal("NewServiceManager() returned nil manager, want non-nil")
		}

		// Test if default OperationTimeout is set
		expectedDefaultTimeout := 30 * time.Second
		if sm.config.OperationTimeout != expectedDefaultTimeout {
			t.Errorf("sm.config.OperationTimeout = %v, want %v", sm.config.OperationTimeout, expectedDefaultTimeout)
		}

		// Test if other config values are preserved
		if sm.config.WatchInterval != cfg.WatchInterval {
			t.Errorf("sm.config.WatchInterval = %v, want %v", sm.config.WatchInterval, cfg.WatchInterval)
		}
	})

	t.Run("DbusConnectionError", func(t *testing.T) {
		expectedErr := errors.New("mock dbus connection error")
		newDbusConnectionFunc = func() (dbusConnInterface, error) {
			return nil, expectedErr
		}

		_, err := NewServiceManager(nil, ManagerConfig{})
		if err == nil {
			t.Fatalf("NewServiceManager() error = nil, wantErr %v", expectedErr)
		}
		if !errors.Is(err, expectedErr) {
			t.Errorf("NewServiceManager() error = %v, want to wrap %v", err, expectedErr)
		}
	})
}

func TestServiceManager_processNodeRecursive(t *testing.T) {
	// Dummy ServiceManager, dbusConn and config are not used by processNodeRecursive directly
	sm := &ServiceManager{}

	tests := []struct {
		name          string
		node          *ServiceNode
		expectedCalls int
		leafNames     []string // Store names of leaves where action was called
	}{
		{
			name:          "NilNode",
			node:          nil, // Should not happen in practice with current StartTree, but good to be robust
			expectedCalls: 0,
		},
		{
			name:          "SingleLeafNode",
			node:          &ServiceNode{Name: "Leaf1", SystemdUnit: "leaf1.service", IsLeaf: true},
			expectedCalls: 1,
			leafNames:     []string{"Leaf1"},
		},
		{
			name:          "SingleNonLeafNodeWithNoChildren",
			node:          &ServiceNode{Name: "NonLeaf1", IsLeaf: false},
			expectedCalls: 0,
		},
		{
			name:          "LeafNodeWithoutSystemdUnit",
			node:          &ServiceNode{Name: "LeafNoUnit", IsLeaf: true /* SystemdUnit is empty */},
			expectedCalls: 0, // leafAction should not be called if SystemdUnit is empty
		},
		{
			name: "TreeWithMultipleLeaves",
			node: &ServiceNode{
				Name:   "Root",
				IsLeaf: false,
				Children: []*ServiceNode{
					{Name: "ChildNonLeaf", IsLeaf: false, Children: []*ServiceNode{
						{Name: "GrandChildLeaf1", SystemdUnit: "gc1.service", IsLeaf: true},
					}},
					{Name: "ChildLeaf1", SystemdUnit: "cl1.service", IsLeaf: true},
					{Name: "ChildLeaf2NoUnit", IsLeaf: true},
				},
			},
			expectedCalls: 2,
			leafNames:     []string{"GrandChildLeaf1", "ChildLeaf1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ( // Use a slice for actualCalls to check order if necessary, for now just names
				mu          sync.Mutex
				calledNames []string
			)

			leafAction := func(ctx context.Context, sn *ServiceNode) {
				mu.Lock()
				calledNames = append(calledNames, sn.Name)
				mu.Unlock()
			}

			// processNodeRecursive can be called with a nil node if sm.RootNodes contains nil,
			// or a node's Children array contains nil. The function should be robust to this.
			if tt.node == nil { // Guard for test setup, actual function should handle it.
				sm.processNodeRecursive(context.Background(), nil, leafAction)
			} else {
				sm.processNodeRecursive(context.Background(), tt.node, leafAction)
			}

			if len(calledNames) != tt.expectedCalls {
				t.Errorf("leafAction called %d times, want %d", len(calledNames), tt.expectedCalls)
			}

			if tt.expectedCalls > 0 && !reflect.DeepEqual(calledNames, tt.leafNames) {
				t.Errorf("leafAction called for nodes %v, want %v", calledNames, tt.leafNames)
			}
		})
	}
}

func TestServiceManager_StartTree(t *testing.T) {
	originalNewDbusConnFunc := newDbusConnectionFunc
	defer func() { newDbusConnectionFunc = originalNewDbusConnFunc }()

	ctx, cancelMainTestCtx := context.WithTimeout(context.Background(), 5*time.Second) // Timeout for the whole test
	defer cancelMainTestCtx()

	cfg := ManagerConfig{
		WatchInterval:    100 * time.Millisecond, // Short interval for testing
		MaxRestarts:      1,
		RestartBackoff:   10 * time.Millisecond,
		OperationTimeout: 500 * time.Millisecond,
	}

	leafService1 := &ServiceNode{Name: "Leaf1", SystemdUnit: "leaf1.service", IsLeaf: true}
	leafService2 := &ServiceNode{Name: "Leaf2", SystemdUnit: "leaf2.service", IsLeaf: true}
	rootNode := &ServiceNode{
		Name:     "Root",
		IsLeaf:   false,
		Children: []*ServiceNode{leafService1, leafService2},
	}

	sm := &ServiceManager{
		RootNodes: []*ServiceNode{rootNode},
		config:    cfg,
		// dbusConn will be set via the mock newDbusConnectionFunc
	}

	t.Run("StartTree_InitialActivationAndWatcherSetup", func(t *testing.T) {
		mockConn := newMockDbusConnection()
		newDbusConnectionFunc = func() (dbusConnInterface, error) {
			return mockConn, nil
		}

		// unitStartedSuccessfully tracks if StartUnit completed for a unit
		unitStartedSuccessfully := make(map[string]bool)
		var muStartSuccess sync.Mutex // Mutex to protect unitStartedSuccessfully

		// Setup mock GetUnitProperties to return services as inactive first, then active
		mockConn.mockGetUnitProperties = func(unit string) (map[string]interface{}, error) {
			mockConn.muGetPropertyCalls.Lock()
			mockConn.getPropertyCalls[unit]++ // Track actual calls
			currentCallCount := mockConn.getPropertyCalls[unit]
			mockConn.muGetPropertyCalls.Unlock()

			muStartSuccess.Lock()
			started := unitStartedSuccessfully[unit]
			muStartSuccess.Unlock()

			var stateToReturn string
			if started {
				stateToReturn = "active"
			} else {
				stateToReturn = "inactive"
			}
			t.Logf("[TestMock] GetUnitProperties for %s (call #%d): unitStartedSuccessfully=%t, returning: %s", unit, currentCallCount, started, stateToReturn)
			return map[string]interface{}{"ActiveState": stateToReturn}, nil
		}

		// Setup mock StartUnit to simulate successful start
		mockConn.mockStartUnit = func(name string, mode string, ch chan<- string) (int, error) {
			mockConn.muStartUnitCalls.Lock()
			mockConn.startUnitCalls[name]++
			currentCallCount := mockConn.startUnitCalls[name]
			mockConn.muStartUnitCalls.Unlock()
			t.Logf("[TestMock] StartUnit for %s (call #%d)", name, currentCallCount)
			muStartSuccess.Lock()
			unitStartedSuccessfully[name] = true // Mark as successfully started
			muStartSuccess.Unlock()
			go func() {
				t.Logf("[TestMock] StartUnit for %s completed, sending 'done' on channel", name)
				ch <- "done"
			}()
			return 1, nil
		}

		sm.dbusConn = mockConn // Assign the mock connection to the service manager instance

		sm.StartTree(ctx) // This will start watchers in goroutines

		// Allow some time for asynchronous operations initiated by StartTree to complete.
		// The OperationTimeout and RestartBackoff are small, so this should be enough for initial calls.
		time.Sleep(cfg.OperationTimeout + cfg.RestartBackoff*2) // Wait a bit longer than a single operation

		// Assertions
		for _, service := range []*ServiceNode{leafService1, leafService2} {
			t.Run(service.Name, func(t *testing.T) {
				service.mu.Lock()
				cancelFuncPresent := service.cancelWatcher != nil
				service.mu.Unlock()

				if !cancelFuncPresent {
					t.Errorf("Expected cancelWatcher to be set for %s, but it was nil", service.Name)
				}

				mockConn.muGetPropertyCalls.Lock()
				getPropertyCallsActual := mockConn.getPropertyCalls[service.SystemdUnit]
				mockConn.muGetPropertyCalls.Unlock()
				if getPropertyCallsActual < 2 { // Should be 2: one initial check, one after start attempt
					t.Errorf("Expected GetUnitProperties to be called at least 2 times for %s, got %d", service.SystemdUnit, getPropertyCallsActual)
				}

				mockConn.muStartUnitCalls.Lock()
				startUnitCallsActual := mockConn.startUnitCalls[service.SystemdUnit]
				mockConn.muStartUnitCalls.Unlock()
				if startUnitCallsActual != 1 {
					t.Errorf("Expected StartUnit to be called once for %s, got %d", service.SystemdUnit, startUnitCallsActual)
				}
			})
		}

		// Important: Clean up by stopping the tree to terminate watcher goroutines
		// StopTree now has its own context handling, ensure it runs to completion.
		sm.StopTree() // Call StopTree to cancel watchers and wait for them
		// Wait for sm.wg.Wait() to complete. This needs to be observable or sm.StopTree needs to be blocking regarding sm.wg.
		// Since StopTree() internally calls sm.wg.Wait(), we just need to ensure StopTree itself has finished.
		// The original StopTree is blocking after initiating all cancellations and then waiting on sm.wg.

		mockConn.closeCalled = false // Reset for other tests if sm is reused, though it's not here.
	})

	// TODO: Add more scenarios for StartTree, e.g., service already active, start fails, etc.
}

// mockDbusConnection is a mock for dbus.Conn
// We are not embedding dbus.Conn because it's a struct with many unexported fields,
// making it hard to use as a base for a simple mock. We only implement methods we use.
type mockDbusConnection struct {
	closeCalled           bool
	getPropertyCalls      map[string]int
	muGetPropertyCalls    sync.Mutex // Protects getPropertyCalls
	startUnitCalls        map[string]int
	muStartUnitCalls      sync.Mutex // Protects startUnitCalls
	stopUnitCalls         map[string]int
	restartUnitCalls      map[string]int
	mockClose             func()
	mockGetUnitProperties func(unit string) (map[string]interface{}, error)
	mockStartUnit         func(name string, mode string, ch chan<- string) (int, error)
	mockStopUnit          func(name string, mode string, ch chan<- string) (int, error)
	mockRestartUnit       func(name string, mode string, ch chan<- string) (int, error)
}

func newMockDbusConnection() *mockDbusConnection {
	return &mockDbusConnection{
		getPropertyCalls: make(map[string]int),
		startUnitCalls:   make(map[string]int),
		stopUnitCalls:    make(map[string]int),
		restartUnitCalls: make(map[string]int),
		mockGetUnitProperties: func(unit string) (map[string]interface{}, error) {
			return map[string]interface{}{"ActiveState": "active"}, nil // Default to active
		},
		mockStartUnit: func(name string, mode string, ch chan<- string) (int, error) {
			go func() { ch <- "done" }()
			return 1, nil
		},
		mockStopUnit: func(name string, mode string, ch chan<- string) (int, error) {
			go func() { ch <- "done" }()
			return 1, nil
		},
		mockRestartUnit: func(name string, mode string, ch chan<- string) (int, error) {
			go func() { ch <- "done" }()
			return 1, nil
		},
	}
}

func (m *mockDbusConnection) Close() {
	m.closeCalled = true
	if m.mockClose != nil {
		m.mockClose()
	}
	// Default mock behavior: do nothing, as original Close() doesn't return error
}

func (m *mockDbusConnection) GetUnitProperties(unit string) (map[string]interface{}, error) {
	// The assigned m.mockGetUnitProperties function will handle counting and specific logic.
	if m.mockGetUnitProperties != nil {
		return m.mockGetUnitProperties(unit)
	}
	// Default behavior or error if not set, though tests should always set it.
	return map[string]interface{}{"ActiveState": "unknown"}, fmt.Errorf("mockGetUnitProperties not set for unit %s", unit)
}

func (m *mockDbusConnection) StartUnit(name string, mode string, ch chan<- string) (int, error) {
	// The assigned m.mockStartUnit function will handle counting and specific logic.
	if m.mockStartUnit != nil {
		return m.mockStartUnit(name, mode, ch)
	}
	return 0, fmt.Errorf("mockStartUnit not set")
}

func (m *mockDbusConnection) StopUnit(name string, mode string, ch chan<- string) (int, error) {
	// The assigned m.mockStopUnit function will handle counting and specific logic.
	if m.mockStopUnit != nil {
		return m.mockStopUnit(name, mode, ch)
	}
	return 0, fmt.Errorf("mockStopUnit not set")
}

func (m *mockDbusConnection) RestartUnit(name string, mode string, ch chan<- string) (int, error) {
	// The assigned m.mockRestartUnit function will handle counting and specific logic.
	if m.mockRestartUnit != nil {
		return m.mockRestartUnit(name, mode, ch)
	}
	return 0, fmt.Errorf("mockRestartUnit not set")
}

// Implement other dbus.Conn methods if they become necessary for tests.
// For example, ListUnits, Subscribe, Unsubscribe, etc.
// For now, these are the ones directly used or passed to systemdOperation.
