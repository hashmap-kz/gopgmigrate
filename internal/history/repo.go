package history

import (
	"context"
	"database/sql"
	"regexp"

	"gopgmigrate/internal/version"

	"gopgmigrate/internal/dbms"
)

type MigrateHistoryRepository interface {
	CreateHistoryTable(ctx context.Context, tx dbms.Transaction) error

	SaveVersioned(ctx context.Context, tx dbms.Transaction, inputEntity *MigrateHistoryCreateInput) error
	SaveRepeatable(ctx context.Context, tx dbms.Transaction, inputEntity *MigrateHistoryCreateInput) error
	ListAll(ctx context.Context, tx dbms.Transaction) ([]MigrateHistory, error)
	DeleteVersion(ctx context.Context, tx dbms.Transaction, v int64) error

	AcquireMigrationLock(ctx context.Context, db dbms.Transaction) (bool, error)
	ReleaseMigrationLock(ctx context.Context, db dbms.Transaction) error

	GetNoTxPatterns() map[string]*regexp.Regexp

	// migration

	RunMigrationsPlainMode(
		ctx context.Context,
		db *sql.DB,
		pendingMigrations []version.MigrationFile,
		directionDo bool,
	) error
}
