package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// HealthHistoryStore manages persistent health history
type HealthHistoryStore struct {
	db       *sql.DB
	dbPath   string
	enabled  bool
}

// HealthEvent represents a health check event
type HealthEvent struct {
	ID            int64
	Timestamp     time.Time
	Service       string
	ServiceType   string
	Chain         string
	IsHealthy     bool
	ActiveState   string
	SubState      string
	CPUPercent    float64
	MemoryBytes   int64
	DiskReadBPS   float64
	DiskWriteBPS  float64
	PeerCount     int
	IsSynced      bool
	RestartCount  int
	ErrorMessage  string
	Metadata      map[string]string
}

// NewHealthHistoryStore creates a new health history store
func NewHealthHistoryStore(dbPath string, enabled bool) (*HealthHistoryStore, error) {
	if !enabled {
		return &HealthHistoryStore{enabled: false}, nil
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	store := &HealthHistoryStore{
		db:      db,
		dbPath:  dbPath,
		enabled: true,
	}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	log.Printf("Health history store initialized: %s", dbPath)
	return store, nil
}

// initSchema creates the database schema
func (h *HealthHistoryStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS health_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		service TEXT NOT NULL,
		service_type TEXT NOT NULL,
		chain TEXT,
		is_healthy BOOLEAN NOT NULL,
		active_state TEXT,
		sub_state TEXT,
		cpu_percent REAL,
		memory_bytes INTEGER,
		disk_read_bps REAL,
		disk_write_bps REAL,
		peer_count INTEGER,
		is_synced BOOLEAN,
		restart_count INTEGER,
		error_message TEXT,
		metadata TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_health_events_timestamp ON health_events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_health_events_service ON health_events(service);
	CREATE INDEX IF NOT EXISTS idx_health_events_service_timestamp ON health_events(service, timestamp);
	CREATE INDEX IF NOT EXISTS idx_health_events_healthy ON health_events(is_healthy, timestamp);

	CREATE TABLE IF NOT EXISTS service_downtime (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		service TEXT NOT NULL,
		start_time DATETIME NOT NULL,
		end_time DATETIME,
		duration_seconds INTEGER,
		reason TEXT,
		resolved BOOLEAN DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_downtime_service ON service_downtime(service);
	CREATE INDEX IF NOT EXISTS idx_downtime_resolved ON service_downtime(resolved);

	CREATE TABLE IF NOT EXISTS restart_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		service TEXT NOT NULL,
		reason TEXT,
		success BOOLEAN NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_restart_service ON restart_events(service);
	CREATE INDEX IF NOT EXISTS idx_restart_timestamp ON restart_events(timestamp);

	CREATE TABLE IF NOT EXISTS alert_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		alert_type TEXT NOT NULL,
		severity TEXT NOT NULL,
		service TEXT NOT NULL,
		message TEXT NOT NULL,
		resolved BOOLEAN DEFAULT 0,
		resolved_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_alert_service ON alert_history(service);
	CREATE INDEX IF NOT EXISTS idx_alert_resolved ON alert_history(resolved);
	CREATE INDEX IF NOT EXISTS idx_alert_timestamp ON alert_history(timestamp);
	`

	_, err := h.db.Exec(schema)
	return err
}

// RecordHealthEvent records a health check event
func (h *HealthHistoryStore) RecordHealthEvent(event HealthEvent) error {
	if !h.enabled {
		return nil
	}

	event.Timestamp = time.Now()

	// Serialize metadata
	var metadataJSON []byte
	var err error
	if len(event.Metadata) > 0 {
		metadataJSON, err = json.Marshal(event.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	query := `
		INSERT INTO health_events (
			timestamp, service, service_type, chain, is_healthy,
			active_state, sub_state, cpu_percent, memory_bytes,
			disk_read_bps, disk_write_bps, peer_count, is_synced,
			restart_count, error_message, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := h.db.Exec(query,
		event.Timestamp, event.Service, event.ServiceType, event.Chain,
		event.IsHealthy, event.ActiveState, event.SubState,
		event.CPUPercent, event.MemoryBytes,
		event.DiskReadBPS, event.DiskWriteBPS,
		event.PeerCount, event.IsSynced,
		event.RestartCount, event.ErrorMessage, metadataJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to insert health event: %w", err)
	}

	id, _ := result.LastInsertId()
	event.ID = id

	return nil
}

// RecordDowntime records a service downtime event
func (h *HealthHistoryStore) RecordDowntime(service, reason string, startTime time.Time) (int64, error) {
	if !h.enabled {
		return 0, nil
	}

	query := `INSERT INTO service_downtime (service, start_time, reason) VALUES (?, ?, ?)`
	result, err := h.db.Exec(query, service, startTime, reason)
	if err != nil {
		return 0, fmt.Errorf("failed to record downtime: %w", err)
	}

	return result.LastInsertId()
}

// ResolveDowntime marks a downtime event as resolved
func (h *HealthHistoryStore) ResolveDowntime(id int64, endTime time.Time) error {
	if !h.enabled {
		return nil
	}

	query := `
		UPDATE service_downtime
		SET end_time = ?, duration_seconds = ?, resolved = 1
		WHERE id = ?
	`

	// Calculate duration from stored start_time
	var startTime time.Time
	err := h.db.QueryRow("SELECT start_time FROM service_downtime WHERE id = ?", id).Scan(&startTime)
	if err != nil {
		return fmt.Errorf("failed to get start time: %w", err)
	}

	duration := int(endTime.Sub(startTime).Seconds())

	_, err = h.db.Exec(query, endTime, duration, id)
	if err != nil {
		return fmt.Errorf("failed to resolve downtime: %w", err)
	}

	return nil
}

// RecordRestart records a service restart event
func (h *HealthHistoryStore) RecordRestart(service, reason string, success bool) error {
	if !h.enabled {
		return nil
	}

	query := `INSERT INTO restart_events (timestamp, service, reason, success) VALUES (?, ?, ?, ?)`
	_, err := h.db.Exec(query, time.Now(), service, reason, success)
	if err != nil {
		return fmt.Errorf("failed to record restart: %w", err)
	}

	return nil
}

// RecordAlert records an alert event
func (h *HealthHistoryStore) RecordAlert(alertType, severity, service, message string) (int64, error) {
	if !h.enabled {
		return 0, nil
	}

	query := `
		INSERT INTO alert_history (timestamp, alert_type, severity, service, message)
		VALUES (?, ?, ?, ?, ?)
	`

	result, err := h.db.Exec(query, time.Now(), alertType, severity, service, message)
	if err != nil {
		return 0, fmt.Errorf("failed to record alert: %w", err)
	}

	return result.LastInsertId()
}

// ResolveAlert marks an alert as resolved
func (h *HealthHistoryStore) ResolveAlert(id int64) error {
	if !h.enabled {
		return nil
	}

	query := `UPDATE alert_history SET resolved = 1, resolved_at = ? WHERE id = ?`
	_, err := h.db.Exec(query, time.Now(), id)
	return err
}

// GetServiceHistory returns health history for a service
func (h *HealthHistoryStore) GetServiceHistory(service string, since time.Time, limit int) ([]HealthEvent, error) {
	if !h.enabled {
		return nil, fmt.Errorf("health history store is disabled")
	}

	if limit == 0 {
		limit = 100
	}

	query := `
		SELECT id, timestamp, service, service_type, chain, is_healthy,
		       active_state, sub_state, cpu_percent, memory_bytes,
		       disk_read_bps, disk_write_bps, peer_count, is_synced,
		       restart_count, error_message, metadata
		FROM health_events
		WHERE service = ? AND timestamp >= ?
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := h.db.Query(query, service, since, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query health events: %w", err)
	}
	defer rows.Close()

	events := make([]HealthEvent, 0, limit)
	for rows.Next() {
		var event HealthEvent
		var metadataJSON sql.NullString

		err := rows.Scan(
			&event.ID, &event.Timestamp, &event.Service, &event.ServiceType,
			&event.Chain, &event.IsHealthy, &event.ActiveState, &event.SubState,
			&event.CPUPercent, &event.MemoryBytes,
			&event.DiskReadBPS, &event.DiskWriteBPS,
			&event.PeerCount, &event.IsSynced,
			&event.RestartCount, &event.ErrorMessage, &metadataJSON,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan health event: %w", err)
		}

		// Deserialize metadata
		if metadataJSON.Valid && metadataJSON.String != "" {
			if err := json.Unmarshal([]byte(metadataJSON.String), &event.Metadata); err != nil {
				log.Printf("Warning: failed to unmarshal metadata: %v", err)
			}
		}

		events = append(events, event)
	}

	return events, rows.Err()
}

// GetServiceUptime calculates service uptime percentage
func (h *HealthHistoryStore) GetServiceUptime(service string, since time.Time) (float64, error) {
	if !h.enabled {
		return 0, fmt.Errorf("health history store is disabled")
	}

	query := `
		SELECT
			SUM(CASE WHEN is_healthy = 1 THEN 1 ELSE 0 END) as healthy_count,
			COUNT(*) as total_count
		FROM health_events
		WHERE service = ? AND timestamp >= ?
	`

	var healthyCount, totalCount int
	err := h.db.QueryRow(query, service, since).Scan(&healthyCount, &totalCount)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate uptime: %w", err)
	}

	if totalCount == 0 {
		return 100.0, nil
	}

	uptime := (float64(healthyCount) / float64(totalCount)) * 100.0
	return uptime, nil
}

// GetDowntimeStats returns downtime statistics
func (h *HealthHistoryStore) GetDowntimeStats(service string, since time.Time) (DowntimeStats, error) {
	var stats DowntimeStats

	if !h.enabled {
		return stats, fmt.Errorf("health history store is disabled")
	}

	query := `
		SELECT
			COUNT(*) as incident_count,
			COALESCE(SUM(duration_seconds), 0) as total_downtime,
			COALESCE(AVG(duration_seconds), 0) as avg_downtime,
			COALESCE(MAX(duration_seconds), 0) as max_downtime
		FROM service_downtime
		WHERE service = ? AND start_time >= ?
	`

	err := h.db.QueryRow(query, service, since).Scan(
		&stats.IncidentCount,
		&stats.TotalDowntimeSeconds,
		&stats.AvgDowntimeSeconds,
		&stats.MaxDowntimeSeconds,
	)

	stats.Service = service
	return stats, err
}

// DowntimeStats holds downtime statistics
type DowntimeStats struct {
	Service              string
	IncidentCount        int
	TotalDowntimeSeconds int
	AvgDowntimeSeconds   float64
	MaxDowntimeSeconds   int
}

// PurgeOldData removes data older than the specified duration
func (h *HealthHistoryStore) PurgeOldData(olderThan time.Duration) error {
	if !h.enabled {
		return nil
	}

	cutoff := time.Now().Add(-olderThan)

	tables := []string{"health_events", "service_downtime", "restart_events", "alert_history"}
	for _, table := range tables {
		query := fmt.Sprintf("DELETE FROM %s WHERE timestamp < ?", table)
		result, err := h.db.Exec(query, cutoff)
		if err != nil {
			return fmt.Errorf("failed to purge %s: %w", table, err)
		}

		rows, _ := result.RowsAffected()
		if rows > 0 {
			log.Printf("Purged %d old rows from %s (older than %v)", rows, table, olderThan)
		}
	}

	// Vacuum to reclaim space
	_, err := h.db.Exec("VACUUM")
	if err != nil {
		log.Printf("Warning: VACUUM failed: %v", err)
	}

	return nil
}

// Close closes the database connection
func (h *HealthHistoryStore) Close() error {
	if h.db != nil {
		return h.db.Close()
	}
	return nil
}
