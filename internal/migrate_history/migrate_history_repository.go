package migrate_history

import (
	"context"
	"database/sql"
)

type MigrateHistoryRepository interface {
	CreateHistoryTable(ctx context.Context, tx *sql.Tx) error
	SaveVersioned(ctx context.Context, tx *sql.Tx, inputEntity *MigrateHistoryVersionedCreateInput) error
	SaveRepeatable(ctx context.Context, tx *sql.Tx, inputEntity *MigrateHistoryVersionedCreateInput) error
	SaveVersionedNoTx(ctx context.Context, conn *sql.DB, inputEntity *MigrateHistoryVersionedCreateInput) error
	SaveRepeatableNoTx(ctx context.Context, conn *sql.DB, inputEntity *MigrateHistoryVersionedCreateInput) error
	ListAll(ctx context.Context, tx *sql.Tx) ([]MigrateHistory, error)
}
