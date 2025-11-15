package main

import (
	"context"
	"fmt"
	"log"
	"time"
)

// AlertRule defines a condition that triggers an alert
type AlertRule struct {
	Name        string
	Type        AlertType
	Severity    AlertSeverity
	Description string
	Evaluator   func(ctx context.Context, service string, data interface{}) (bool, string)
	Enabled     bool
}

// AlertRuleEngine evaluates alert rules and fires alerts
type AlertRuleEngine struct {
	rules        []AlertRule
	alertManager *AlertManager
	metrics      *MetricsCollector
}

// NewAlertRuleEngine creates a new alert rule engine
func NewAlertRuleEngine(alertManager *AlertManager, metrics *MetricsCollector) *AlertRuleEngine {
	engine := &AlertRuleEngine{
		rules:        make([]AlertRule, 0),
		alertManager: alertManager,
		metrics:      metrics,
	}

	// Register default rules
	engine.registerDefaultRules()

	return engine
}

// registerDefaultRules sets up the default alert rules
func (e *AlertRuleEngine) registerDefaultRules() {
	e.AddRule(AlertRule{
		Name:        "HighCPUUsage",
		Type:        AlertHighCPU,
		Severity:    SeverityWarning,
		Description: "CPU usage exceeds 80%",
		Enabled:     true,
		Evaluator: func(ctx context.Context, service string, data interface{}) (bool, string) {
			usage, ok := data.(*ResourceUsage)
			if !ok {
				return false, ""
			}
			if usage.CPUPercent > 80.0 {
				return true, fmt.Sprintf("CPU usage at %.1f%%", usage.CPUPercent)
			}
			return false, ""
		},
	})

	e.AddRule(AlertRule{
		Name:        "CriticalCPUUsage",
		Type:        AlertHighCPU,
		Severity:    SeverityCritical,
		Description: "CPU usage exceeds 95%",
		Enabled:     true,
		Evaluator: func(ctx context.Context, service string, data interface{}) (bool, string) {
			usage, ok := data.(*ResourceUsage)
			if !ok {
				return false, ""
			}
			if usage.CPUPercent > 95.0 {
				return true, fmt.Sprintf("Critical CPU usage at %.1f%%", usage.CPUPercent)
			}
			return false, ""
		},
	})

	e.AddRule(AlertRule{
		Name:        "HighMemoryUsage",
		Type:        AlertHighMemory,
		Severity:    SeverityWarning,
		Description: "Memory usage exceeds 2GB",
		Enabled:     true,
		Evaluator: func(ctx context.Context, service string, data interface{}) (bool, string) {
			usage, ok := data.(*ResourceUsage)
			if !ok {
				return false, ""
			}
			const twoGB = 2 * 1024 * 1024 * 1024
			if usage.MemoryBytes > twoGB {
				memGB := float64(usage.MemoryBytes) / (1024 * 1024 * 1024)
				return true, fmt.Sprintf("Memory usage at %.2f GB", memGB)
			}
			return false, ""
		},
	})

	e.AddRule(AlertRule{
		Name:        "CriticalMemoryUsage",
		Type:        AlertHighMemory,
		Severity:    SeverityCritical,
		Description: "Memory usage exceeds 4GB",
		Enabled:     true,
		Evaluator: func(ctx context.Context, service string, data interface{}) (bool, string) {
			usage, ok := data.(*ResourceUsage)
			if !ok {
				return false, ""
			}
			const fourGB = 4 * 1024 * 1024 * 1024
			if usage.MemoryBytes > fourGB {
				memGB := float64(usage.MemoryBytes) / (1024 * 1024 * 1024)
				return true, fmt.Sprintf("Critical memory usage at %.2f GB", memGB)
			}
			return false, ""
		},
	})

	e.AddRule(AlertRule{
		Name:        "HighDiskIO",
		Type:        AlertHighDiskIO,
		Severity:    SeverityWarning,
		Description: "Disk I/O exceeds 100 MB/s",
		Enabled:     true,
		Evaluator: func(ctx context.Context, service string, data interface{}) (bool, string) {
			usage, ok := data.(*ResourceUsage)
			if !ok {
				return false, ""
			}
			const hundredMB = 100 * 1024 * 1024
			totalIO := usage.DiskReadBPS + usage.DiskWriteBPS
			if totalIO > hundredMB {
				ioMB := totalIO / (1024 * 1024)
				return true, fmt.Sprintf("Disk I/O at %.2f MB/s (read: %.2f, write: %.2f)",
					ioMB, usage.DiskReadBPS/(1024*1024), usage.DiskWriteBPS/(1024*1024))
			}
			return false, ""
		},
	})

	e.AddRule(AlertRule{
		Name:        "RestartLoop",
		Type:        AlertRestartLoop,
		Severity:    SeverityCritical,
		Description: "Service restarted 3+ times in 5 minutes",
		Enabled:     true,
		Evaluator: func(ctx context.Context, service string, data interface{}) (bool, string) {
			state, ok := e.metrics.GetServiceState(service)
			if !ok {
				return false, ""
			}
			// This is a simplified check - in production you'd track restart timestamps
			if state.RestartCount >= 3 {
				return true, fmt.Sprintf("Service restarted %d times", state.RestartCount)
			}
			return false, ""
		},
	})

	e.AddRule(AlertRule{
		Name:        "LowPeerCount",
		Type:        AlertLowPeerCount,
		Severity:    SeverityWarning,
		Description: "Node has fewer than 5 peers",
		Enabled:     true,
		Evaluator: func(ctx context.Context, service string, data interface{}) (bool, string) {
			// This would check peer count from sync status
			// For now, this is a placeholder
			return false, ""
		},
	})

	e.AddRule(AlertRule{
		Name:        "ServiceDown",
		Type:        AlertServiceDown,
		Severity:    SeverityCritical,
		Description: "Service is not running",
		Enabled:     true,
		Evaluator: func(ctx context.Context, service string, data interface{}) (bool, string) {
			status, ok := data.(*SystemdServiceStatus)
			if !ok {
				return false, ""
			}
			if !status.IsActive {
				return true, fmt.Sprintf("Service is %s/%s", status.ActiveState, status.SubState)
			}
			return false, ""
		},
	})

	e.AddRule(AlertRule{
		Name:        "HealthCheckFailed",
		Type:        AlertHealthCheckFailed,
		Severity:    SeverityWarning,
		Description: "HTTP health check failed",
		Enabled:     true,
		Evaluator: func(ctx context.Context, service string, data interface{}) (bool, string) {
			result, ok := data.(*HTTPHealthCheckResult)
			if !ok {
				return false, ""
			}
			if !result.Healthy {
				return true, fmt.Sprintf("Health check failed: %s", result.Error)
			}
			return false, ""
		},
	})
}

// AddRule adds a custom alert rule
func (e *AlertRuleEngine) AddRule(rule AlertRule) {
	e.rules = append(e.rules, rule)
	log.Printf("Registered alert rule: %s (%s/%s)", rule.Name, rule.Type, rule.Severity)
}

// Evaluate evaluates all rules for a service and data
func (e *AlertRuleEngine) Evaluate(ctx context.Context, service string, data interface{}) {
	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}

		triggered, message := rule.Evaluator(ctx, service, data)
		if triggered {
			alert := Alert{
				Type:      rule.Type,
				Severity:  rule.Severity,
				Service:   service,
				Message:   message,
				Timestamp: time.Now(),
				Labels: map[string]string{
					"rule": rule.Name,
				},
				Annotations: map[string]string{
					"description": rule.Description,
				},
			}

			if err := e.alertManager.FireAlert(ctx, alert); err != nil {
				log.Printf("Failed to fire alert for rule %s: %v", rule.Name, err)
			}
		}
	}
}

// EvaluateResourceUsage is a convenience method for resource usage data
func (e *AlertRuleEngine) EvaluateResourceUsage(ctx context.Context, service string, usage *ResourceUsage) {
	e.Evaluate(ctx, service, usage)
}

// EvaluateServiceStatus is a convenience method for service status data
func (e *AlertRuleEngine) EvaluateServiceStatus(ctx context.Context, service string, status *SystemdServiceStatus) {
	e.Evaluate(ctx, service, status)
}

// EvaluateHealthCheck is a convenience method for health check results
func (e *AlertRuleEngine) EvaluateHealthCheck(ctx context.Context, service string, result *HTTPHealthCheckResult) {
	e.Evaluate(ctx, service, result)
}

// DisableRule disables a rule by name
func (e *AlertRuleEngine) DisableRule(name string) {
	for i := range e.rules {
		if e.rules[i].Name == name {
			e.rules[i].Enabled = false
			log.Printf("Disabled alert rule: %s", name)
			return
		}
	}
}

// EnableRule enables a rule by name
func (e *AlertRuleEngine) EnableRule(name string) {
	for i := range e.rules {
		if e.rules[i].Name == name {
			e.rules[i].Enabled = true
			log.Printf("Enabled alert rule: %s", name)
			return
		}
	}
}

// GetRules returns all registered rules
func (e *AlertRuleEngine) GetRules() []AlertRule {
	rules := make([]AlertRule, len(e.rules))
	copy(rules, e.rules)
	return rules
}
