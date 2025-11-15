package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// AlertSeverity represents the severity level of an alert
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// AlertType represents different types of alerts
type AlertType string

const (
	AlertServiceDown       AlertType = "service_down"
	AlertServiceDegraded   AlertType = "service_degraded"
	AlertHighCPU           AlertType = "high_cpu"
	AlertHighMemory        AlertType = "high_memory"
	AlertHighDiskIO        AlertType = "high_disk_io"
	AlertRestartLoop       AlertType = "restart_loop"
	AlertSyncStalled       AlertType = "sync_stalled"
	AlertLowPeerCount      AlertType = "low_peer_count"
	AlertDependencyTimeout AlertType = "dependency_timeout"
	AlertHealthCheckFailed AlertType = "health_check_failed"
)

// Alert represents an alert event
type Alert struct {
	Type        AlertType
	Severity    AlertSeverity
	Service     string
	Message     string
	Timestamp   time.Time
	Labels      map[string]string
	Annotations map[string]string
}

// AlertChannel represents a destination for alerts
type AlertChannel interface {
	Send(ctx context.Context, alert Alert) error
	Name() string
}

// AlertManager manages alert routing and deduplication
type AlertManager struct {
	channels        []AlertChannel
	activeAlerts    map[string]*Alert // key = alert fingerprint
	alertHistory    []Alert
	mu              sync.RWMutex
	metrics         *MetricsCollector
	dedupeWindow    time.Duration
	maxHistorySize  int
}

// NewAlertManager creates a new alert manager
func NewAlertManager(metrics *MetricsCollector, dedupeWindow time.Duration) *AlertManager {
	return &AlertManager{
		channels:       make([]AlertChannel, 0),
		activeAlerts:   make(map[string]*Alert),
		alertHistory:   make([]Alert, 0),
		metrics:        metrics,
		dedupeWindow:   dedupeWindow,
		maxHistorySize: 1000,
	}
}

// RegisterChannel adds an alert channel
func (am *AlertManager) RegisterChannel(channel AlertChannel) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.channels = append(am.channels, channel)
	log.Printf("Registered alert channel: %s", channel.Name())
}

// FireAlert sends an alert through all registered channels
func (am *AlertManager) FireAlert(ctx context.Context, alert Alert) error {
	alert.Timestamp = time.Now()

	// Generate fingerprint for deduplication
	fingerprint := am.generateFingerprint(alert)

	am.mu.Lock()

	// Check if this is a duplicate within the dedupe window
	if existingAlert, exists := am.activeAlerts[fingerprint]; exists {
		if time.Since(existingAlert.Timestamp) < am.dedupeWindow {
			am.mu.Unlock()
			log.Printf("Alert deduplicated: %s for %s (within %v window)",
				alert.Type, alert.Service, am.dedupeWindow)
			return nil
		}
	}

	// Mark as active
	am.activeAlerts[fingerprint] = &alert

	// Add to history
	am.alertHistory = append(am.alertHistory, alert)
	if len(am.alertHistory) > am.maxHistorySize {
		am.alertHistory = am.alertHistory[len(am.alertHistory)-am.maxHistorySize:]
	}

	channels := make([]AlertChannel, len(am.channels))
	copy(channels, am.channels)

	am.mu.Unlock()

	// Update metrics
	if am.metrics != nil {
		am.metrics.RecordAlertFired(string(alert.Type), string(alert.Severity), alert.Service)
		am.updateActiveAlertMetrics()
	}

	log.Printf("Firing alert: [%s] %s - %s: %s",
		alert.Severity, alert.Type, alert.Service, alert.Message)

	// Send through all channels
	var lastErr error
	for _, channel := range channels {
		if err := channel.Send(ctx, alert); err != nil {
			log.Printf("Failed to send alert via %s: %v", channel.Name(), err)
			lastErr = err
		}
	}

	return lastErr
}

// ResolveAlert marks an alert as resolved
func (am *AlertManager) ResolveAlert(alert Alert) {
	fingerprint := am.generateFingerprint(alert)

	am.mu.Lock()
	delete(am.activeAlerts, fingerprint)
	am.mu.Unlock()

	if am.metrics != nil {
		am.updateActiveAlertMetrics()
	}

	log.Printf("Alert resolved: [%s] %s - %s",
		alert.Severity, alert.Type, alert.Service)
}

// GetActiveAlerts returns all currently active alerts
func (am *AlertManager) GetActiveAlerts() []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	alerts := make([]Alert, 0, len(am.activeAlerts))
	for _, alert := range am.activeAlerts {
		alerts = append(alerts, *alert)
	}
	return alerts
}

// GetAlertHistory returns recent alert history
func (am *AlertManager) GetAlertHistory(limit int) []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if limit == 0 || limit > len(am.alertHistory) {
		limit = len(am.alertHistory)
	}

	start := len(am.alertHistory) - limit
	history := make([]Alert, limit)
	copy(history, am.alertHistory[start:])
	return history
}

// generateFingerprint creates a unique key for an alert
func (am *AlertManager) generateFingerprint(alert Alert) string {
	return fmt.Sprintf("%s:%s:%s", alert.Type, alert.Service, alert.Severity)
}

// updateActiveAlertMetrics updates Prometheus metrics for active alerts
func (am *AlertManager) updateActiveAlertMetrics() {
	// Count alerts by type and severity
	counts := make(map[string]map[string]int)

	am.mu.RLock()
	for _, alert := range am.activeAlerts {
		typeKey := string(alert.Type)
		sevKey := string(alert.Severity)

		if counts[typeKey] == nil {
			counts[typeKey] = make(map[string]int)
		}
		counts[typeKey][sevKey]++
	}
	am.mu.RUnlock()

	// Update metrics
	for alertType, severities := range counts {
		for severity, count := range severities {
			am.metrics.RecordActiveAlerts(alertType, severity, count)
		}
	}
}

// LogChannel logs alerts to the application log
type LogChannel struct{}

func NewLogChannel() *LogChannel {
	return &LogChannel{}
}

func (c *LogChannel) Name() string {
	return "log"
}

func (c *LogChannel) Send(ctx context.Context, alert Alert) error {
	log.Printf("[ALERT] [%s] %s - %s: %s",
		alert.Severity, alert.Type, alert.Service, alert.Message)
	return nil
}

// WebhookChannel sends alerts to a webhook URL
type WebhookChannel struct {
	url     string
	headers map[string]string
	client  *http.Client
}

func NewWebhookChannel(url string, headers map[string]string) *WebhookChannel {
	return &WebhookChannel{
		url:     url,
		headers: headers,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *WebhookChannel) Name() string {
	return fmt.Sprintf("webhook:%s", c.url)
}

func (c *WebhookChannel) Send(ctx context.Context, alert Alert) error {
	payload := map[string]interface{}{
		"type":      alert.Type,
		"severity":  alert.Severity,
		"service":   alert.Service,
		"message":   alert.Message,
		"timestamp": alert.Timestamp.Unix(),
		"labels":    alert.Labels,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal alert: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// SlackChannel sends alerts to Slack via webhook
type SlackChannel struct {
	webhookURL string
	client     *http.Client
}

func NewSlackChannel(webhookURL string) *SlackChannel {
	return &SlackChannel{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *SlackChannel) Name() string {
	return "slack"
}

func (c *SlackChannel) Send(ctx context.Context, alert Alert) error {
	color := ""
	switch alert.Severity {
	case SeverityInfo:
		color = "#36a64f" // green
	case SeverityWarning:
		color = "#ff9900" // orange
	case SeverityCritical:
		color = "#ff0000" // red
	}

	payload := map[string]interface{}{
		"attachments": []map[string]interface{}{
			{
				"color":      color,
				"title":      fmt.Sprintf("[%s] %s", alert.Severity, alert.Type),
				"text":       alert.Message,
				"footer":     alert.Service,
				"ts":         alert.Timestamp.Unix(),
				"mrkdwn_in":  []string{"text"},
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("Slack request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Slack returned status %d", resp.StatusCode)
	}

	return nil
}

// EmailChannel sends alerts via email (placeholder - would need SMTP config)
type EmailChannel struct {
	smtpHost string
	smtpPort int
	from     string
	to       []string
}

func NewEmailChannel(smtpHost string, smtpPort int, from string, to []string) *EmailChannel {
	return &EmailChannel{
		smtpHost: smtpHost,
		smtpPort: smtpPort,
		from:     from,
		to:       to,
	}
}

func (c *EmailChannel) Name() string {
	return "email"
}

func (c *EmailChannel) Send(ctx context.Context, alert Alert) error {
	// TODO: Implement SMTP email sending
	// For now, just log that we would send an email
	log.Printf("[Email] Would send alert to %v: [%s] %s - %s",
		c.to, alert.Severity, alert.Type, alert.Message)
	return nil
}
