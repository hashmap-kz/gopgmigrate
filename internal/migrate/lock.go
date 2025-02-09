package migrate

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Migration Lock Key (must be unique per application)
const migrationLockKey = 123456

// acquireMigrationLock ensures only one migration process runs at a time
func acquireMigrationLock(ctx context.Context, conn *pgx.Conn) (bool, error) {
	var acquired bool
	err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", migrationLockKey).Scan(&acquired)
	return acquired, err
}

// releaseMigrationLock releases the advisory lock
func releaseMigrationLock(ctx context.Context, conn *pgx.Conn) error {
	_, err := conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", migrationLockKey)
	return err
}
