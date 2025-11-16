package main

import (
	"log"

	"github.com/coreos/go-systemd/v22/dbus"
)

// Activities are the external operations that workflows can call
// They handle actual interactions with systemd and other services

type Activities struct {
	dbusConn       *dbus.Conn // Kept for backward compatibility
	processManager ProcessManager
	executeMode    bool // true = execute actions, false = dry-run (watch only)
	metrics        *MetricsCollector
	alertManager   *AlertManager
	alertEngine    *AlertRuleEngine
	enableResourceMonitoring bool
	circuitBreakers *CircuitBreakerManager
	healthHistory   *HealthHistoryStore
	dynamicConfig   *DynamicConfig
	database        Database // Database interface for batch and cron operations
}

func NewActivities(executeMode bool, metrics *MetricsCollector, alertManager *AlertManager, enableResourceMonitoring bool, cbManager *CircuitBreakerManager, healthHistory *HealthHistoryStore, dynamicConfig *DynamicConfig, processManager ProcessManager) (*Activities, error) {
	// Keep D-Bus connection for backward compatibility (can be removed later)
	conn, err := dbus.New()
	if err != nil {
		log.Printf("Warning: failed to connect to D-Bus (continuing with process manager): %v", err)
	}

	mode := "watch (dry-run)"
	if executeMode {
		mode = "exec (execute actions)"
	}
	log.Printf("Activities initialized in %s mode (process manager: %s)", mode, processManager.Name())

	activities := &Activities{
		dbusConn:       conn,
		processManager: processManager,
		executeMode:    executeMode,
		metrics:        metrics,
		alertManager:   alertManager,
		enableResourceMonitoring: enableResourceMonitoring,
		circuitBreakers: cbManager,
		healthHistory:   healthHistory,
		dynamicConfig:   dynamicConfig,
	}

	// Create alert engine if alerting is enabled
	if alertManager != nil {
		activities.alertEngine = NewAlertRuleEngine(alertManager, metrics)
	}

	return activities, nil
}

func (a *Activities) Close() {
	if a.dbusConn != nil {
		a.dbusConn.Close()
	}
	if a.processManager != nil {
		a.processManager.Close()
	}
	if a.healthHistory != nil {
		a.healthHistory.Close()
	}
	if a.database != nil {
		a.database.Close()
	}
}

// SetDatabase sets the database for batch and cron operations
func (a *Activities) SetDatabase(db Database) {
	a.database = db
}
