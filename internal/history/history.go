package history

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// required to load pgx
	_ "github.com/jackc/pgx/v5/stdlib"
)

const advisoryLockKey = 8273645019 // arbitrary stable key for this tool

// tx abstracts *sql.DB and *sql.Tx so repo methods work in both contexts.
type tx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Row is a single record in the tracking table.
type Row struct {
	RecordID    int
	MigrationID string
	Path        string
	Kind        string
	Checksum    string
	Description string
	AppliedBy   string
	AppliedAt   time.Time
	TxID        string
}

type repo struct {
	table string
}

func newRepo(table string) *repo {
	return &repo{table: table}
}

func (r *repo) createTable(ctx context.Context, db tx) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf(`
		create table if not exists %s (
			record_id    serial      primary key,
			migration_id text        not null unique,
			path         text        not null,
			kind         text        not null,
			checksum     text        not null,
			description  text,
			applied_by   name        not null default session_user,
			applied_at   timestamptz not null default transaction_timestamp(),
			txid         text        not null default pg_current_xact_id()::text
		)
	`, r.table))
	if err != nil {
		return fmt.Errorf("history: create table: %w", err)
	}
	return nil
}

func (r *repo) loadAll(ctx context.Context, db tx) (map[string]Row, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`
		select record_id, migration_id, path, kind, checksum, coalesce(description,''), applied_by, applied_at, txid
		from %s
	`, r.table))
	if err != nil {
		return nil, fmt.Errorf("history: list: %w", err)
	}
	defer rows.Close()

	out := make(map[string]Row)
	for rows.Next() {
		var row Row
		if err := rows.Scan(
			&row.RecordID, &row.MigrationID, &row.Path, &row.Kind, &row.Checksum, &row.Description,
			&row.AppliedBy, &row.AppliedAt, &row.TxID,
		); err != nil {
			return nil, fmt.Errorf("history: scan: %w", err)
		}
		out[row.Path] = row
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("history: rows: %w", err)
	}
	return out, nil
}

func (r *repo) insert(ctx context.Context, db tx, migrationID, path, kind, checksum, description string) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf(`
		insert into %s (migration_id, path, kind, checksum, description)
		values ($1, $2, $3, $4, $5)
	`, r.table), migrationID, path, kind, checksum, description)
	if err != nil {
		return fmt.Errorf("history: insert %q: %w", migrationID, err)
	}
	return nil
}

func (r *repo) upsert(ctx context.Context, db tx, migrationID, path, kind, checksum, description string) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf(`
		insert into %s (migration_id, path, kind, checksum, description)
		values ($1, $2, $3, $4, $5)
		on conflict (migration_id) do update
			set checksum    = excluded.checksum,
			    description = excluded.description,
			    applied_by  = session_user,
			    applied_at  = transaction_timestamp(),
			    txid        = pg_current_xact_id()::text
	`, r.table), migrationID, path, kind, checksum, description)
	if err != nil {
		return fmt.Errorf("history: upsert %q: %w", migrationID, err)
	}
	return nil
}

func acquireLock(ctx context.Context, db tx) (bool, error) {
	var acquired bool
	err := db.QueryRowContext(ctx, "select pg_try_advisory_lock($1)", advisoryLockKey).Scan(&acquired)
	return acquired, err
}

func releaseLock(ctx context.Context, db tx) error {
	_, err := db.ExecContext(ctx, "select pg_advisory_unlock($1)", advisoryLockKey)
	return err
}
