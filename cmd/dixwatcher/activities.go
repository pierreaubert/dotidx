package main

import (
	"fmt"
	"log"

	"github.com/coreos/go-systemd/v22/dbus"
)

// Activities are the external operations that workflows can call
// They handle actual interactions with systemd and other services

type Activities struct {
	dbusConn    *dbus.Conn
	executeMode bool // true = execute actions, false = dry-run (watch only)
}

func NewActivities(executeMode bool) (*Activities, error) {
	conn, err := dbus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to D-Bus: %w", err)
	}

	mode := "watch (dry-run)"
	if executeMode {
		mode = "exec (execute actions)"
	}
	log.Printf("Activities initialized in %s mode", mode)

	return &Activities{
		dbusConn:    conn,
		executeMode: executeMode,
	}, nil
}

func (a *Activities) Close() {
	if a.dbusConn != nil {
		a.dbusConn.Close()
	}
}
