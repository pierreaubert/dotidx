package main

import (
	"context"
	"time"

	"github.com/pierreaubert/dotidx/dix"
)

// Database is an interface wrapper around dix.Database
// This allows us to use the database in activities
type Database interface {
	GetExistingBlocks(relayChain, chain string, startRange, endRange int) (map[int]bool, error)
	Save(items []dix.BlockData, relayChain, chain string) error
	GetDatabaseInfo() ([]dix.DatabaseInfo, error)
	ReadTimeNamedQuery(ctx context.Context, relayChain, chain, queryName string, year, month int) (time.Time, error)
	ExecuteAndStoreNamedQuery(ctx context.Context, relayChain, chain, queryName string, year, month int) error
	Close() error
}

// DixDatabaseAdapter adapts dix.Database to our Database interface
type DixDatabaseAdapter struct {
	db dix.Database
}

// NewDixDatabaseAdapter creates a new adapter
func NewDixDatabaseAdapter(db dix.Database) Database {
	return &DixDatabaseAdapter{db: db}
}

func (d *DixDatabaseAdapter) GetExistingBlocks(relayChain, chain string, startRange, endRange int) (map[int]bool, error) {
	return d.db.GetExistingBlocks(relayChain, chain, startRange, endRange)
}

func (d *DixDatabaseAdapter) Save(items []dix.BlockData, relayChain, chain string) error {
	return d.db.Save(items, relayChain, chain)
}

func (d *DixDatabaseAdapter) GetDatabaseInfo() ([]dix.DatabaseInfo, error) {
	return d.db.GetDatabaseInfo()
}

func (d *DixDatabaseAdapter) ReadTimeNamedQuery(ctx context.Context, relayChain, chain, queryName string, year, month int) (time.Time, error) {
	return d.db.ReadTimeNamedQuery(ctx, relayChain, chain, queryName, year, month)
}

func (d *DixDatabaseAdapter) ExecuteAndStoreNamedQuery(ctx context.Context, relayChain, chain, queryName string, year, month int) error {
	return d.db.ExecuteAndStoreNamedQuery(ctx, relayChain, chain, queryName, year, month)
}

func (d *DixDatabaseAdapter) Close() error {
	return d.db.Close()
}
