package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
)

// Activities are the external operations that workflows can call
// They handle actual interactions with systemd and other services

type Activities struct {
	dbusConn *dbus.Conn
}

func NewActivities() (*Activities, error) {
	conn, err := dbus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to D-Bus: %w", err)
	}
	return &Activities{dbusConn: conn}, nil
}

func (a *Activities) Close() {
	if a.dbusConn != nil {
		a.dbusConn.Close()
	}
}

// SystemdServiceStatus represents the status of a systemd service
type SystemdServiceStatus struct {
	IsActive    bool
	ActiveState string
	SubState    string
	LoadState   string
}

// CheckSystemdServiceActivity checks if a systemd service is running and healthy
func (a *Activities) CheckSystemdServiceActivity(ctx context.Context, unitName string) (*SystemdServiceStatus, error) {
	log.Printf("[Activity] Checking systemd service: %s", unitName)

	props, err := a.dbusConn.GetUnitPropertiesContext(ctx, unitName)
	if err != nil {
		return nil, fmt.Errorf("failed to get properties for %s: %w", unitName, err)
	}

	activeState, ok := props["ActiveState"].(string)
	if !ok {
		return nil, fmt.Errorf("ActiveState for %s is not a string or not found", unitName)
	}

	subState, _ := props["SubState"].(string)
	loadState, _ := props["LoadState"].(string)

	status := &SystemdServiceStatus{
		IsActive:    activeState == "active",
		ActiveState: activeState,
		SubState:    subState,
		LoadState:   loadState,
	}

	log.Printf("[Activity] Service %s status: ActiveState=%s, SubState=%s, LoadState=%s",
		unitName, status.ActiveState, status.SubState, status.LoadState)

	return status, nil
}

// StartSystemdServiceActivity starts a systemd service
func (a *Activities) StartSystemdServiceActivity(ctx context.Context, unitName string) error {
	log.Printf("[Activity] Starting systemd service: %s", unitName)

	reschan := make(chan string)
	_, err := a.dbusConn.StartUnitContext(ctx, unitName, "replace", reschan)
	if err != nil {
		return fmt.Errorf("failed to start service %s: %w", unitName, err)
	}

	// Wait for result with timeout
	select {
	case result := <-reschan:
		if result == "done" {
			log.Printf("[Activity] Successfully started service: %s", unitName)
			return nil
		}
		return fmt.Errorf("start operation for %s finished with result: %s", unitName, result)
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for start operation on %s: %w", unitName, ctx.Err())
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for start operation on %s", unitName)
	}
}

// StopSystemdServiceActivity stops a systemd service
func (a *Activities) StopSystemdServiceActivity(ctx context.Context, unitName string) error {
	log.Printf("[Activity] Stopping systemd service: %s", unitName)

	reschan := make(chan string)
	_, err := a.dbusConn.StopUnitContext(ctx, unitName, "replace", reschan)
	if err != nil {
		return fmt.Errorf("failed to stop service %s: %w", unitName, err)
	}

	// Wait for result with timeout
	select {
	case result := <-reschan:
		if result == "done" {
			log.Printf("[Activity] Successfully stopped service: %s", unitName)
			return nil
		}
		return fmt.Errorf("stop operation for %s finished with result: %s", unitName, result)
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for stop operation on %s: %w", unitName, ctx.Err())
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for stop operation on %s", unitName)
	}
}

// RestartSystemdServiceActivity restarts a systemd service
func (a *Activities) RestartSystemdServiceActivity(ctx context.Context, unitName string) error {
	log.Printf("[Activity] Restarting systemd service: %s", unitName)

	reschan := make(chan string)
	_, err := a.dbusConn.RestartUnitContext(ctx, unitName, "replace", reschan)
	if err != nil {
		return fmt.Errorf("failed to restart service %s: %w", unitName, err)
	}

	// Wait for result with timeout
	select {
	case result := <-reschan:
		if result == "done" {
			log.Printf("[Activity] Successfully restarted service: %s", unitName)
			return nil
		}
		return fmt.Errorf("restart operation for %s finished with result: %s", unitName, result)
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for restart operation on %s: %w", unitName, ctx.Err())
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for restart operation on %s", unitName)
	}
}
