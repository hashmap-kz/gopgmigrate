package migrator

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"

	"github.com/hashmap-kz/gopgmigrate/v2/internal/conn"
	"github.com/hashmap-kz/gopgmigrate/v2/internal/executor"
	"github.com/hashmap-kz/gopgmigrate/v2/internal/manifest"
)

// Config holds options for the migrator.
// Table defaults to "schema_migrations" when empty.
// Output, when non-nil, receives a progress table during Run. Pass os.Stdout for CLI use.
type Config struct {
	Dir    string
	Table  string
	DryRun bool
	Output io.Writer
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
// dsn may be empty when standard PG* environment variables are configured
// (PGHOST, PGDATABASE, PGUSER, etc.) - pgx reads them automatically.
// Returns an error early if neither dsn nor any PG* env var is set.
func NewWithDSN(dsn string, cfg Config) (*Migrator, error) {
	if dsn == "" && !hasPGEnv() {
		return nil, fmt.Errorf("migrator: no connection configured:" +
			" provide a DSN or set PGHOST / PGPORT / PGDATABASE / PGUSER")
	}
	db, err := conn.Open(dsn)
	if err != nil {
		return nil, err
	}
	return &Migrator{db: db, ownDB: true, cfg: cfg}, nil
}

// hasPGEnv reports whether any standard PostgreSQL connection environment
// variable is set. When true, pgx can build a connection without an explicit DSN.
func hasPGEnv() bool {
	for _, key := range []string{"PGHOST", "PGPORT", "PGDATABASE", "PGUSER", "PGPASSWORD"} {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
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
	return executor.Run(ctx, m.db, mf, m.cfg.Output, m.cfg.DryRun)
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

// NewValidateOnly creates a Migrator for validation only - no DB connection needed.
func NewValidateOnly(cfg Config) (*Migrator, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("migrator: Dir is required")
	}
	return &Migrator{cfg: cfg}, nil
}

func (m *Migrator) loadManifest() (*manifest.Manifest, error) {
	if m.cfg.Dir == "" {
		return nil, fmt.Errorf("migrator: Dir is required")
	}
	mf, err := manifest.Scan(m.cfg.Dir)
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
