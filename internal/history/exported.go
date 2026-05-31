package history

import (
	"context"
	"database/sql"
)

// Record holds the fields written to the history table for a single applied migration.
type Record struct {
	MigrationID int64
	Path        string
	Kind        string
	Checksum    string
}

// Exported is the only type accessible outside this package.
// It wraps repo and exposes exactly what the executor needs.
type Exported struct {
	r *repo
}

func NewExported(table string) *Exported {
	return &Exported{r: newRepo(table)}
}

func (e *Exported) Init(ctx context.Context, db *sql.DB) error {
	return e.r.createTable(ctx, db)
}

func (e *Exported) All(ctx context.Context, db *sql.DB) (map[string]Row, error) {
	return e.r.loadAll(ctx, db)
}

func (e *Exported) Insert(ctx context.Context, db tx, rec *Record) error {
	return e.r.insert(ctx, db, rec)
}

func (e *Exported) Upsert(ctx context.Context, db tx, rec *Record) error {
	return e.r.upsert(ctx, db, rec)
}

func (e *Exported) Lock(ctx context.Context, db *sql.DB) (bool, error) {
	return acquireLock(ctx, db)
}

func (e *Exported) Unlock(ctx context.Context, db *sql.DB) error {
	return releaseLock(ctx, db)
}
