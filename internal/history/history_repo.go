package history

import (
	"context"
	"database/sql"
)

type MigrateHistoryRepository interface {
	CreateHistoryTable(ctx context.Context, tx *sql.Tx) error
	SaveVersioned(ctx context.Context, tx *sql.Tx, inputEntity *MigrateHistoryCreateInput) error
	SaveRepeatable(ctx context.Context, tx *sql.Tx, inputEntity *MigrateHistoryCreateInput) error
	SaveVersionedNoTx(ctx context.Context, conn *sql.DB, inputEntity *MigrateHistoryCreateInput) error
	SaveRepeatableNoTx(ctx context.Context, conn *sql.DB, inputEntity *MigrateHistoryCreateInput) error
	ListAll(ctx context.Context, tx *sql.Tx) ([]MigrateHistory, error)
}
