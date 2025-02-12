package migrate

import (
	"context"
	"database/sql"

	"gopgmigrate/internal/history"
)

// RunMigrationsGroupMode applies all pending migrations, wrapping in TX (for tx-based), or no-tx (for *.ntx.)
func RunMigrationsGroupMode(
	ctx context.Context,
	db *sql.DB,
	repo history.MigrateHistoryRepository,
	group GroupEntry,
	directionDo bool,
) error {
	return migrateListOfFilesFn(ctx, db, group.Files, group.UseTX, repo, directionDo)
}
