package main

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsCollector manages Prometheus metrics for dixwatcher
type MetricsCollector struct {
	// Service health metrics
	serviceHealth *prometheus.GaugeVec
	serviceRestarts *prometheus.CounterVec
	serviceDowntime *prometheus.CounterVec

	// Resource metrics
	serviceCPU *prometheus.GaugeVec
	serviceMemory *prometheus.GaugeVec
	serviceDiskIO *prometheus.GaugeVec

	// Workflow metrics
	workflowExecutions *prometheus.CounterVec
	workflowDuration *prometheus.HistogramVec
	activityExecutions *prometheus.CounterVec
	activityDuration *prometheus.HistogramVec
	activityErrors *prometheus.CounterVec

	// Sync metrics
	nodeSyncStatus *prometheus.GaugeVec
	nodePeerCount *prometheus.GaugeVec

	// Dependency metrics
	dependencyWaitTime *prometheus.HistogramVec
	dependencyTimeouts *prometheus.CounterVec

	// Alert metrics
	alertsFired *prometheus.CounterVec
	alertsActive *prometheus.GaugeVec

	mu sync.RWMutex
	serviceStates map[string]ServiceMetricState
}

// ServiceMetricState tracks state for a service
type ServiceMetricState struct {
	LastHealthy time.Time
	LastUnhealthy time.Time
	TotalDowntime time.Duration
	RestartCount int
}

// NewMetricsCollector creates and initializes a new metrics collector
func NewMetricsCollector(namespace string) *MetricsCollector {
	mc := &MetricsCollector{
		serviceStates: make(map[string]ServiceMetricState),

		// Service health metrics
		serviceHealth: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "service_health",
				Help:      "Current health status of services (1=healthy, 0=unhealthy)",
			},
			[]string{"service", "type", "chain"},
		),

		serviceRestarts: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "service_restarts_total",
				Help:      "Total number of service restarts",
			},
			[]string{"service", "type"},
		),

		serviceDowntime: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "service_downtime_seconds_total",
				Help:      "Total downtime in seconds",
			},
			[]string{"service", "type"},
		),

		// Resource metrics
		serviceCPU: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "service_cpu_percent",
				Help:      "CPU usage percentage",
			},
			[]string{"service", "type"},
		),

		serviceMemory: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "service_memory_bytes",
				Help:      "Memory usage in bytes",
			},
			[]string{"service", "type"},
		),

		serviceDiskIO: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "service_disk_io_bytes_per_second",
				Help:      "Disk I/O rate in bytes per second",
			},
			[]string{"service", "type", "direction"},
		),

		// Workflow metrics
		workflowExecutions: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "workflow_executions_total",
				Help:      "Total number of workflow executions",
			},
			[]string{"workflow", "status"},
		),

		workflowDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "workflow_duration_seconds",
				Help:      "Workflow execution duration",
				Buckets:   prometheus.ExponentialBuckets(1, 2, 10), // 1s to ~17min
			},
			[]string{"workflow"},
		),

		activityExecutions: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "activity_executions_total",
				Help:      "Total number of activity executions",
			},
			[]string{"activity", "status"},
		),

		activityDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "activity_duration_seconds",
				Help:      "Activity execution duration",
				Buckets:   prometheus.ExponentialBuckets(0.1, 2, 8), // 100ms to ~25s
			},
			[]string{"activity"},
		),

		activityErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "activity_errors_total",
				Help:      "Total number of activity errors",
			},
			[]string{"activity", "error_type"},
		),

		// Sync metrics
		nodeSyncStatus: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "node_sync_status",
				Help:      "Node sync status (1=synced, 0=syncing)",
			},
			[]string{"node", "chain"},
		),

		nodePeerCount: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "node_peer_count",
				Help:      "Number of connected peers",
			},
			[]string{"node", "chain"},
		),

		// Dependency metrics
		dependencyWaitTime: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "dependency_wait_time_seconds",
				Help:      "Time spent waiting for dependencies",
				Buckets:   prometheus.ExponentialBuckets(1, 2, 12), // 1s to ~1hr
			},
			[]string{"service", "dependency"},
		),

		dependencyTimeouts: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "dependency_timeouts_total",
				Help:      "Total number of dependency timeouts",
			},
			[]string{"service", "dependency"},
		),

		// Alert metrics
		alertsFired: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "alerts_fired_total",
				Help:      "Total number of alerts fired",
			},
			[]string{"alert_type", "severity", "service"},
		),

		alertsActive: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "alerts_active",
				Help:      "Number of currently active alerts",
			},
			[]string{"alert_type", "severity"},
		),
	}

	return mc
}

// RecordServiceHealth records the health status of a service
func (mc *MetricsCollector) RecordServiceHealth(service, serviceType, chain string, healthy bool) {
	healthValue := 0.0
	if healthy {
		healthValue = 1.0
	}
	mc.serviceHealth.WithLabelValues(service, serviceType, chain).Set(healthValue)

	mc.mu.Lock()
	defer mc.mu.Unlock()

	state := mc.serviceStates[service]
	now := time.Now()

	if healthy {
		if !state.LastUnhealthy.IsZero() && state.LastHealthy.Before(state.LastUnhealthy) {
			// Was unhealthy, now healthy - calculate downtime
			downtime := now.Sub(state.LastUnhealthy)
			state.TotalDowntime += downtime
			mc.serviceDowntime.WithLabelValues(service, serviceType).Add(downtime.Seconds())
		}
		state.LastHealthy = now
	} else {
		state.LastUnhealthy = now
	}

	mc.serviceStates[service] = state
}

// RecordServiceRestart records a service restart
func (mc *MetricsCollector) RecordServiceRestart(service, serviceType string) {
	mc.serviceRestarts.WithLabelValues(service, serviceType).Inc()

	mc.mu.Lock()
	defer mc.mu.Unlock()

	state := mc.serviceStates[service]
	state.RestartCount++
	mc.serviceStates[service] = state
}

// RecordResourceUsage records resource usage for a service
func (mc *MetricsCollector) RecordResourceUsage(service, serviceType string, cpuPercent, memoryBytes, diskReadBPS, diskWriteBPS float64) {
	mc.serviceCPU.WithLabelValues(service, serviceType).Set(cpuPercent)
	mc.serviceMemory.WithLabelValues(service, serviceType).Set(memoryBytes)
	mc.serviceDiskIO.WithLabelValues(service, serviceType, "read").Set(diskReadBPS)
	mc.serviceDiskIO.WithLabelValues(service, serviceType, "write").Set(diskWriteBPS)
}

// RecordWorkflowExecution records a workflow execution
func (mc *MetricsCollector) RecordWorkflowExecution(workflow, status string) {
	mc.workflowExecutions.WithLabelValues(workflow, status).Inc()
}

// RecordWorkflowDuration records workflow execution duration
func (mc *MetricsCollector) RecordWorkflowDuration(workflow string, duration time.Duration) {
	mc.workflowDuration.WithLabelValues(workflow).Observe(duration.Seconds())
}

// RecordActivityExecution records an activity execution
func (mc *MetricsCollector) RecordActivityExecution(activity, status string) {
	mc.activityExecutions.WithLabelValues(activity, status).Inc()
}

// RecordActivityDuration records activity execution duration
func (mc *MetricsCollector) RecordActivityDuration(activity string, duration time.Duration) {
	mc.activityDuration.WithLabelValues(activity).Observe(duration.Seconds())
}

// RecordActivityError records an activity error
func (mc *MetricsCollector) RecordActivityError(activity, errorType string) {
	mc.activityErrors.WithLabelValues(activity, errorType).Inc()
}

// RecordNodeSyncStatus records node sync status
func (mc *MetricsCollector) RecordNodeSyncStatus(node, chain string, synced bool, peerCount int) {
	syncValue := 0.0
	if synced {
		syncValue = 1.0
	}
	mc.nodeSyncStatus.WithLabelValues(node, chain).Set(syncValue)
	mc.nodePeerCount.WithLabelValues(node, chain).Set(float64(peerCount))
}

// RecordDependencyWait records time waiting for a dependency
func (mc *MetricsCollector) RecordDependencyWait(service, dependency string, duration time.Duration) {
	mc.dependencyWaitTime.WithLabelValues(service, dependency).Observe(duration.Seconds())
}

// RecordDependencyTimeout records a dependency timeout
func (mc *MetricsCollector) RecordDependencyTimeout(service, dependency string) {
	mc.dependencyTimeouts.WithLabelValues(service, dependency).Inc()
}

// RecordAlertFired records an alert being fired
func (mc *MetricsCollector) RecordAlertFired(alertType, severity, service string) {
	mc.alertsFired.WithLabelValues(alertType, severity, service).Inc()
}

// RecordActiveAlerts records the number of active alerts
func (mc *MetricsCollector) RecordActiveAlerts(alertType, severity string, count int) {
	mc.alertsActive.WithLabelValues(alertType, severity).Set(float64(count))
}

// GetServiceState returns the current state for a service
func (mc *MetricsCollector) GetServiceState(service string) (ServiceMetricState, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	state, ok := mc.serviceStates[service]
	return state, ok
}

// StartMetricsServer starts an HTTP server exposing Prometheus metrics
func (mc *MetricsCollector) StartMetricsServer(addr string) error {
	http.Handle("/metrics", promhttp.Handler())
	return http.ListenAndServe(addr, nil)
}
