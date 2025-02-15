package history

import (
	"context"
	"database/sql"
	"regexp"

	"gopgmigrate/internal/modes"
	"gopgmigrate/internal/vers"

	"gopgmigrate/internal/dbms"
)

type MigrateHistoryRepository interface {
	CreateHistoryTable(ctx context.Context, tx dbms.Transaction) error

	SaveVersioned(ctx context.Context, tx dbms.Transaction, inputEntity *MigrateHistoryCreateInput) error
	SaveRepeatable(ctx context.Context, tx dbms.Transaction, inputEntity *MigrateHistoryCreateInput) error
	ListAll(ctx context.Context, tx dbms.Transaction) ([]MigrateHistory, error)
	DeleteVersion(ctx context.Context, tx dbms.Transaction, v int64) error
	VersionExists(ctx context.Context, tx dbms.Transaction, v int64) (bool, error)

	AcquireMigrationLock(ctx context.Context, db dbms.Transaction) (bool, error)
	ReleaseMigrationLock(ctx context.Context, db dbms.Transaction) error

	GetNoTxPatterns() map[string]*regexp.Regexp

	// migrate

	RunMigrationsPlainMode(
		ctx context.Context,
		db *sql.DB,
		pendingMigrations []vers.MigrationFile,
		directionDo bool,
	) error

	RunMigrationsMixedMode(
		ctx context.Context,
		db *sql.DB,
		groupEntries []modes.GroupEntry,
		directionDo bool,
	) error

	RunMigrationsGroupMode(
		ctx context.Context,
		db *sql.DB,
		groupEntry modes.GroupEntry,
		directionDo bool,
	) error
}
