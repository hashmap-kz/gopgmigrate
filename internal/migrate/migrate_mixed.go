package migrate

import (
	"context"
	"database/sql"
	"fmt"

	"gopgmigrate/internal/history"
)

// RunMigrationsMixedMode applies all pending migrations, wrapping in TX (for tx-based), and no-tx (for *.ntx.)
func RunMigrationsMixedMode(
	ctx context.Context,
	db *sql.DB,
	repo history.MigrateHistoryRepository,
	groups []*GroupEntry,
	directionDo bool,
) error {
	for _, elem := range groups {
		if elem == nil {
			return fmt.Errorf("internal error. unexpected nil group")
		}
		err := migrateListOfFilesFn(ctx, db, elem.Files, elem.UseTX, repo, directionDo)
		if err != nil {
			return err
		}
	}
	return nil
}
