package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// ResourceUsage represents resource usage metrics for a service
type ResourceUsage struct {
	CPUPercent    float64 // CPU usage as percentage (0-100 per core)
	MemoryBytes   int64   // Memory usage in bytes
	DiskReadBPS   float64 // Disk read bytes per second
	DiskWriteBPS  float64 // Disk write bytes per second
	Timestamp     time.Time
}

// CheckResourceUsageActivity monitors resource usage for a systemd service
func (a *Activities) CheckResourceUsageActivity(ctx context.Context, unitName string) (*ResourceUsage, error) {
	log.Printf("[Activity] Checking resource usage for: %s", unitName)

	// Get the main PID of the service
	props, err := a.dbusConn.GetUnitPropertiesContext(ctx, unitName)
	if err != nil {
		return nil, fmt.Errorf("failed to get properties for %s: %w", unitName, err)
	}

	mainPID, ok := props["MainPID"].(uint32)
	if !ok || mainPID == 0 {
		return nil, fmt.Errorf("no MainPID found for %s", unitName)
	}

	// Read CPU and memory stats from /proc
	cpuPercent, err := getCPUUsage(int(mainPID))
	if err != nil {
		log.Printf("[Activity] Warning: failed to get CPU usage for PID %d: %v", mainPID, err)
		cpuPercent = 0
	}

	memoryBytes, err := getMemoryUsage(int(mainPID))
	if err != nil {
		log.Printf("[Activity] Warning: failed to get memory usage for PID %d: %v", mainPID, err)
		memoryBytes = 0
	}

	diskReadBPS, diskWriteBPS, err := getDiskIORate(int(mainPID))
	if err != nil {
		log.Printf("[Activity] Warning: failed to get disk I/O for PID %d: %v", mainPID, err)
		diskReadBPS, diskWriteBPS = 0, 0
	}

	usage := &ResourceUsage{
		CPUPercent:   cpuPercent,
		MemoryBytes:  memoryBytes,
		DiskReadBPS:  diskReadBPS,
		DiskWriteBPS: diskWriteBPS,
		Timestamp:    time.Now(),
	}

	log.Printf("[Activity] Resource usage for %s (PID %d): CPU=%.2f%%, Mem=%d bytes, DiskR=%.2f B/s, DiskW=%.2f B/s",
		unitName, mainPID, usage.CPUPercent, usage.MemoryBytes, usage.DiskReadBPS, usage.DiskWriteBPS)

	return usage, nil
}

// getCPUUsage reads CPU usage from /proc/[pid]/stat
// Returns CPU percentage (0-100 per core)
func getCPUUsage(pid int) (float64, error) {
	// Read current CPU time
	stat1, err := readProcStat(pid)
	if err != nil {
		return 0, err
	}

	// Wait a short interval
	time.Sleep(100 * time.Millisecond)

	// Read CPU time again
	stat2, err := readProcStat(pid)
	if err != nil {
		return 0, err
	}

	// Calculate CPU percentage
	totalTime1 := stat1.utime + stat1.stime
	totalTime2 := stat2.utime + stat2.stime
	elapsed := totalTime2 - totalTime1

	// elapsed is in clock ticks, convert to percentage
	// 100 ticks per second on most systems (USER_HZ)
	cpuPercent := float64(elapsed) / 100.0 * 10.0 // *10 because we waited 100ms

	return cpuPercent, nil
}

type procStat struct {
	utime uint64 // CPU time in user mode (clock ticks)
	stime uint64 // CPU time in kernel mode (clock ticks)
}

func readProcStat(pid int) (*procStat, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return nil, err
	}

	// Parse /proc/[pid]/stat
	// Format: pid (comm) state ppid pgrp session tty_nr tpgid flags minflt cminflt majflt cmajflt utime stime ...
	fields := strings.Fields(string(data))
	if len(fields) < 15 {
		return nil, fmt.Errorf("invalid /proc/%d/stat format", pid)
	}

	utime, err := strconv.ParseUint(fields[13], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse utime: %w", err)
	}

	stime, err := strconv.ParseUint(fields[14], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse stime: %w", err)
	}

	return &procStat{
		utime: utime,
		stime: stime,
	}, nil
}

// getMemoryUsage reads memory usage from /proc/[pid]/status
// Returns RSS (Resident Set Size) in bytes
func getMemoryUsage(pid int) (int64, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err
	}

	// Parse /proc/[pid]/status to find VmRSS
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			// VmRSS is in kB
			kb, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse VmRSS: %w", err)
			}
			return kb * 1024, nil // Convert to bytes
		}
	}

	return 0, fmt.Errorf("VmRSS not found in /proc/%d/status", pid)
}

// getDiskIORate reads disk I/O stats from /proc/[pid]/io
// Returns read and write bytes per second
func getDiskIORate(pid int) (float64, float64, error) {
	// Read first sample
	io1, err := readProcIO(pid)
	if err != nil {
		return 0, 0, err
	}
	time1 := time.Now()

	// Wait a short interval
	time.Sleep(100 * time.Millisecond)

	// Read second sample
	io2, err := readProcIO(pid)
	if err != nil {
		return 0, 0, err
	}
	time2 := time.Now()

	// Calculate rates
	elapsed := time2.Sub(time1).Seconds()
	readBPS := float64(io2.readBytes-io1.readBytes) / elapsed
	writeBPS := float64(io2.writeBytes-io1.writeBytes) / elapsed

	return readBPS, writeBPS, nil
}

type procIO struct {
	readBytes  uint64
	writeBytes uint64
}

func readProcIO(pid int) (*procIO, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/io", pid))
	if err != nil {
		return nil, err
	}

	io := &procIO{}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}

		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}

		switch fields[0] {
		case "read_bytes:":
			io.readBytes = value
		case "write_bytes:":
			io.writeBytes = value
		}
	}

	return io, nil
}
