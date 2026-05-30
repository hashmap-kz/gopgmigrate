package migrator

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hashmap-kz/gopgmigrate/v2/internal/conn"
	"github.com/hashmap-kz/gopgmigrate/v2/internal/executor"
	"github.com/hashmap-kz/gopgmigrate/v2/internal/manifest"
)

// Config holds options for the migrator.
// Table defaults to "schema_migrations" when empty.
type Config struct {
	ManifestPath string
	Table        string
	DryRun       bool
}

// EntryStatus describes the current state of a single migration entry.
type EntryStatus = executor.EntryStatus

// Migrator applies migrations defined in a YAML manifest.
// Create with NewWithDSN or NewWithDB.
type Migrator struct {
	db    *sql.DB
	ownDB bool // true if we opened the connection and must close it
	cfg   Config
}

// NewWithDSN creates a Migrator that opens and owns its own DB connection.
func NewWithDSN(dsn string, cfg Config) (*Migrator, error) {
	if dsn == "" {
		return nil, fmt.Errorf("migrator: dsn is required")
	}
	db, err := conn.Open(dsn)
	if err != nil {
		return nil, err
	}
	return &Migrator{db: db, ownDB: true, cfg: cfg}, nil
}

// NewWithDB creates a Migrator using a caller-managed DB connection.
// The caller remains responsible for closing the connection.
func NewWithDB(db *sql.DB, cfg Config) (*Migrator, error) {
	if db == nil {
		return nil, fmt.Errorf("migrator: db is nil")
	}
	return &Migrator{db: db, ownDB: false, cfg: cfg}, nil
}

// Close releases the DB connection if the Migrator owns it.
func (m *Migrator) Close() error {
	if m.ownDB {
		return m.db.Close()
	}
	return nil
}

// Run applies all pending migrations in manifest order.
func (m *Migrator) Run(ctx context.Context) error {
	mf, err := m.loadManifest()
	if err != nil {
		return err
	}
	return executor.Run(ctx, m.db, mf, m.cfg.DryRun)
}

// Status returns the current state of every entry in the manifest.
func (m *Migrator) Status(ctx context.Context) ([]EntryStatus, error) {
	mf, err := m.loadManifest()
	if err != nil {
		return nil, err
	}
	return executor.Status(ctx, m.db, mf)
}

// Validate checks that all files referenced in the manifest exist
// and are readable. Does not connect to the database.
func (m *Migrator) Validate() error {
	mf, err := m.loadManifest()
	if err != nil {
		return err
	}
	return executor.Validate(mf)
}

// NewValidateOnly creates a Migrator for validation only — no DB connection needed.
func NewValidateOnly(cfg Config) (*Migrator, error) {
	if cfg.ManifestPath == "" {
		return nil, fmt.Errorf("migrator: ManifestPath is required")
	}
	return &Migrator{cfg: cfg}, nil
}

func (m *Migrator) loadManifest() (*manifest.Manifest, error) {
	if m.cfg.ManifestPath == "" {
		return nil, fmt.Errorf("migrator: ManifestPath is required")
	}
	mf, err := manifest.Load(m.cfg.ManifestPath)
	if err != nil {
		return nil, err
	}
	if m.cfg.Table != "" {
		mf.Table = m.cfg.Table
	}
	return mf, nil
}

// NoTxHistoryError is returned by Run when a no-tx migration was applied
// successfully but the history record failed to write.
// Inspect RecoverySQL() to get the exact INSERT needed to recover,
// then re-run after executing it manually.
type NoTxHistoryError = executor.NoTxHistoryError
