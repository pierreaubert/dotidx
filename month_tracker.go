package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// MonthTracker tracks the completion status of blocks by month
type MonthTracker struct {
	db            Database
	config        Config
	monthProgress map[string]MonthProgress
	mutex         sync.RWMutex
	dumpDir       string
}

// MonthProgress tracks the progress of blocks within a month
type MonthProgress struct {
	Month       string // YYYY-MM format
	StartBlock  int
	EndBlock    int
	Processed   int
	Expected    int
	DumpCreated bool
}

// NewMonthTracker creates a new month tracker
func NewMonthTracker(db Database, config Config, dumpDir string) *MonthTracker {
	return &MonthTracker{
		db:            db,
		config:        config,
		monthProgress: make(map[string]MonthProgress),
		dumpDir:       dumpDir,
	}
}

// GetMonthKey returns the month key (YYYY-MM) for a given timestamp
func GetMonthKey(timestamp time.Time) string {
	return fmt.Sprintf("%04d-%02d", timestamp.Year(), timestamp.Month())
}

// GetBlockMonth retrieves the month key for a given block ID
func (m *MonthTracker) GetBlockMonth(blockID int, reader ChainReader) (string, error) {
	// Get block data to check timestamp
	blockData, err := reader.FetchBlock(context.Background(), blockID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch block %d: %w", blockID, err)
	}
	
	return GetMonthKey(blockData.Timestamp), nil
}

// InitializeMonthRanges initializes the month ranges for the given block range
func (m *MonthTracker) InitializeMonthRanges(startBlock, endBlock int, reader ChainReader) error {
	// Sample blocks at regular intervals to determine month boundaries
	sampleSize := 100
	step := (endBlock - startBlock) / sampleSize
	if step < 1 {
		step = 1
	}
	
	monthBoundaries := make(map[string]struct{
		First int
		Last  int
	})
	
	for blockID := startBlock; blockID <= endBlock; blockID += step {
		monthKey, err := m.GetBlockMonth(blockID, reader)
		if err != nil {
			log.Printf("Warning: Could not determine month for block %d: %v", blockID, err)
			continue
		}
		
		info, exists := monthBoundaries[monthKey]
		if !exists || blockID < info.First {
			info.First = blockID
		}
		if !exists || blockID > info.Last {
			info.Last = blockID
		}
		monthBoundaries[monthKey] = info
	}
	
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// Create MonthProgress entries for each month
	for month, boundaries := range monthBoundaries {
		estimatedBlocksInMonth := boundaries.Last - boundaries.First + 1
		m.monthProgress[month] = MonthProgress{
			Month:       month,
			StartBlock:  boundaries.First,
			EndBlock:    boundaries.Last,
			Processed:   0,
			Expected:    estimatedBlocksInMonth,
			DumpCreated: false,
		}
	}
	
	return nil
}

// UpdateProgress updates the progress for a specific month based on processed blocks
func (m *MonthTracker) UpdateProgress(month string, addedBlocks int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	progress, exists := m.monthProgress[month]
	if !exists {
		// If we don't know about this month yet, create a new entry
		progress = MonthProgress{
			Month:       month,
			Processed:   0,
			Expected:    1, // Will be updated later when we get more info
			DumpCreated: false,
		}
	}
	
	progress.Processed += addedBlocks
	m.monthProgress[month] = progress
	
	// Check if this month is now complete
	m.checkAndDumpIfComplete(month)
}

// checkAndDumpIfComplete checks if a month is complete and creates a dump if needed
func (m *MonthTracker) checkAndDumpIfComplete(month string) {
	progress, exists := m.monthProgress[month]
	if !exists {
		return
	}
	
	// If we've already created a dump or the month isn't complete yet, skip
	if progress.DumpCreated || progress.Processed < progress.Expected {
		return
	}
	
	// Create database dump for this month
	if err := m.createDumpForMonth(month); err != nil {
		log.Printf("Error creating dump for month %s: %v", month, err)
		return
	}
	
	// Mark this month as dumped
	progress.DumpCreated = true
	m.monthProgress[month] = progress
	log.Printf("Created database dump for month %s", month)
}

// createDumpForMonth executes pg_dump to create a dump file for the specified month
func (m *MonthTracker) createDumpForMonth(month string) error {
	parts := strings.Split(month, "-")
	if len(parts) != 2 {
		return fmt.Errorf("invalid month format: %s", month)
	}
	
	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid year in month %s: %w", month, err)
	}
	
	monthNum, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid month number in month %s: %w", month, err)
	}
	
	// Parse database connection string to extract components
	dbURL := m.config.DatabaseURL
	dbName, dbHost, dbPort, dbUser, dbPassword := parseDBConnectionString(dbURL)
	
	// Determine the partition suffix for the month
	partitionSuffix := fmt.Sprintf("%d_%02d", year, monthNum)
	
	// Determine the table names to dump
	blocksTable := fmt.Sprintf("chain.blocks_%s_%s_%s", 
		sanitizeChainName(m.config.Relaychain, ""),
		sanitizeChainName(m.config.Chain, m.config.Relaychain),
		partitionSuffix)
	
	addressTable := fmt.Sprintf("chain.address2blocks_%s_%s_%s", 
		sanitizeChainName(m.config.Relaychain, ""),
		sanitizeChainName(m.config.Chain, m.config.Relaychain),
		partitionSuffix)
	
	// Create dump filename
	dumpFilename := filepath.Join(m.dumpDir, 
		fmt.Sprintf("%s_%s_%s.sql", 
			m.config.Relaychain, 
			m.config.Chain, 
			month))
	
	// Check if we should run pg_dump locally or remotely
	remoteHost := os.Getenv("DOTIDX_DB_HOST")
	remoteUser := os.Getenv("DOTIDX_DB_USER")
	
	log.Printf("Creating database dump for month %s to %s", month, dumpFilename)
	
	var cmd *exec.Cmd
	var args []string
	
	if remoteHost != "" {
		// Remote execution via SSH
		sshTarget := remoteHost
		if remoteUser != "" {
			sshTarget = remoteUser + "@" + remoteHost
		}
		
		// Build pg_dump command to run remotely
		pgDumpCmd := fmt.Sprintf("pg_dump --host %s --port %s --username %s", 
			dbHost, dbPort, dbUser)
		
		if dbPassword != "" {
			// Use password from environment variable to avoid exposing it in the command line
			// This will be picked up by the PGPASSWORD environment variable on the remote host
			pgDumpCmd = fmt.Sprintf("PGPASSWORD='%s' %s", dbPassword, pgDumpCmd)
		}
		
		// Complete the pg_dump command with the specific tables and output redirection
		pgDumpCmd = fmt.Sprintf("%s --dbname=%s -t %s -t %s > %s",
			pgDumpCmd, dbName, blocksTable, addressTable, dumpFilename)
		
		// Run the command via SSH
		cmd = exec.Command("ssh", sshTarget, pgDumpCmd)
	} else {
		// Local execution
		args = []string{
			"--host", dbHost,
			"--port", dbPort,
			"--username", dbUser,
			"--dbname", dbName,
			"-t", blocksTable,
			"-t", addressTable,
			"--file", dumpFilename,
		}
		
		// Build the pg_dump command
		cmd = exec.Command("pg_dump", args...)
		
		// Set password environment variable
		if dbPassword != "" {
			cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", dbPassword))
		}
	}
	
	// Execute the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_dump failed: %w, output: %s", err, string(output))
	}
	
	return nil
}

// parseDBConnectionString extracts components from a PostgreSQL connection string
func parseDBConnectionString(dbURL string) (dbName, host, port, user, password string) {
	// Default values
	dbName = "postgres"
	host = "localhost"
	port = "5432"
	user = "postgres"
	password = ""
	
	// Handle standard postgres:// URL format
	if strings.HasPrefix(dbURL, "postgres://") {
		// Remove the prefix
		dbURL = strings.TrimPrefix(dbURL, "postgres://")
		
		// Split user:password and host:port/dbname
		parts := strings.SplitN(dbURL, "@", 2)
		
		if len(parts) == 2 {
			// Extract user and password
			userParts := strings.SplitN(parts[0], ":", 2)
			user = userParts[0]
			if len(userParts) > 1 {
				password = userParts[1]
			}
			
			// Extract host, port, and database name
			hostDBParts := strings.SplitN(parts[1], "/", 2)
			if len(hostDBParts) > 0 {
				hostPortParts := strings.SplitN(hostDBParts[0], ":", 2)
				host = hostPortParts[0]
				if len(hostPortParts) > 1 {
					port = hostPortParts[1]
				}
			}
			
			if len(hostDBParts) > 1 {
				dbName = hostDBParts[1]
			}
		}
	}
	
	return dbName, host, port, user, password
}

// GetMonthProgress returns the current progress for all months
func (m *MonthTracker) GetMonthProgress() map[string]MonthProgress {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	// Create a copy to avoid concurrent map access issues
	progress := make(map[string]MonthProgress)
	for month, monthProgress := range m.monthProgress {
		progress[month] = monthProgress
	}
	
	return progress
}
