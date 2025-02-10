package history

import (
	"context"
	"database/sql"
)

type MigrateHistoryRepository interface {
	CreateHistoryTable(ctx context.Context, tx *sql.Tx) error

	SaveVersioned(ctx context.Context, tx *sql.Tx, inputEntity *MigrateHistoryCreateInput) error
	SaveVersionedNoTx(ctx context.Context, db *sql.DB, inputEntity *MigrateHistoryCreateInput) error

	SaveRepeatable(ctx context.Context, tx *sql.Tx, inputEntity *MigrateHistoryCreateInput) error
	SaveRepeatableNoTx(ctx context.Context, db *sql.DB, inputEntity *MigrateHistoryCreateInput) error

	ListAll(ctx context.Context, tx *sql.Tx) ([]MigrateHistory, error)

	DeleteVersion(ctx context.Context, tx *sql.Tx, scriptName string) error
	DeleteVersionNoTx(ctx context.Context, db *sql.DB, scriptName string) error

	AcquireMigrationLock(ctx context.Context, db *sql.DB) (bool, error)
	ReleaseMigrationLock(ctx context.Context, db *sql.DB) error
}
