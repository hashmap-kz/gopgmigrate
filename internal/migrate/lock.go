package migrate

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Migration Lock Key (must be unique per application)
const migrationLockKey = 123456

// acquireMigrationLock ensures only one migration process runs at a time
func acquireMigrationLock(tx pgx.Tx) (bool, error) {
	ctx := context.Background()
	var acquired bool
	err := tx.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", migrationLockKey).Scan(&acquired)
	return acquired, err
}

// releaseMigrationLock releases the advisory lock
func releaseMigrationLock(tx pgx.Tx) error {
	_, err := tx.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockKey)
	return err
}
