package history

import (
	"context"
	"database/sql"
)

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

func (e *Exported) Insert(ctx context.Context, db tx, path, kind, checksum, description string) error {
	return e.r.insert(ctx, db, path, kind, checksum, description)
}

func (e *Exported) Upsert(ctx context.Context, db tx, path, kind, checksum, description string) error {
	return e.r.upsert(ctx, db, path, kind, checksum, description)
}

func (e *Exported) Lock(ctx context.Context, db *sql.DB) (bool, error) {
	return acquireLock(ctx, db)
}

func (e *Exported) Unlock(ctx context.Context, db *sql.DB) error {
	return releaseLock(ctx, db)
}
