package migrate

import (
	"context"
	"database/sql"
	"fmt"

	"gopgmigrate/internal/history"
)

// RunMigrationsGroupMode applies all pending migrations, wrapping in TX (for tx-based), or no-tx (for *.ntx.)
func RunMigrationsGroupMode(
	ctx context.Context,
	db *sql.DB,
	repo history.MigrateHistoryRepository,
	group *GroupEntry,
	directionDo bool,
) error {
	if group == nil {
		return fmt.Errorf("internal error. unexpected nil group, func: RunMigrationsGroupMode")
	}
	return migrateListOfFilesFn(ctx, db, group.Files, group.UseTX, repo, directionDo)
}
