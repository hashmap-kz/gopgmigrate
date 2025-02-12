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
	files []MigrationFile,
	useTx bool,
	repo history.MigrateHistoryRepository,
	directionDo bool,
) error {
	return migrateListOfFilesFn(ctx, db, files, useTx, repo, directionDo)
}
