package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	"gopgmigrate/internal/history"
)

func RunMigrations(
	ctx context.Context,
	migrateMode string,
	db *sql.DB,
	repo history.MigrateHistoryRepository,
	pendingMigrations []MigrationFile,
	directionDo bool,
) error {
	// lock

	acquired, err := repo.AcquireMigrationLock(ctx, db)
	if err != nil {
		slog.Error("unable to acquire lock", slog.String("err", err.Error()))
		os.Exit(1)
	}
	if !acquired {
		slog.Error("another migration process is running. exiting.")
		os.Exit(1)
	}
	slog.Debug("lock", slog.String("status", "acquired:true"))
	defer func(ctx context.Context, conn *sql.DB) {
		err = repo.ReleaseMigrationLock(ctx, conn)
		if err != nil {
			slog.Warn("lock", slog.String("status", err.Error()))
		} else {
			slog.Debug("lock", slog.String("status", "released:true"))
		}
	}(ctx, db)

	// migrate

	if migrateMode == ModeMixed {
		return runMigrationsMixedMode(ctx, db, repo, pendingMigrations, directionDo)
	} else if migrateMode == ModePlain {
		return runMigrationsPlainMode(ctx, db, repo, pendingMigrations, directionDo)
	} else if migrateMode == ModeGroup {
		return runMigrationsGroupMode(ctx, db, repo, pendingMigrations, directionDo)
	}
	return fmt.Errorf("unknown mode: %s", migrateMode)
}

// runMigrationsPlainMode applies both versioned and repeatable migrations
func runMigrationsPlainMode(
	ctx context.Context,
	db *sql.DB,
	repo history.MigrateHistoryRepository,
	pendingMigrations []MigrationFile,
	directionDo bool,
) error {
	for _, elem := range pendingMigrations {
		err := migrateOneScript(ctx, db, elem, repo, directionDo)
		if err != nil {
			return err
		}
	}
	return nil
}

// runMigrationsMixedMode applies all pending migrations, wrapping in TX (for tx-based), and no-tx (for *.ntx.)
func runMigrationsMixedMode(
	ctx context.Context,
	db *sql.DB,
	repo history.MigrateHistoryRepository,
	pendingMigrations []MigrationFile,
	directionDo bool,
) error {
	groupEntries, err := ParseFilesMixedMode(pendingMigrations)
	if err != nil {
		return err
	}
	for _, elem := range groupEntries {
		err := migrateListOfFilesFn(ctx, db, elem.Files, elem.UseTX, repo, directionDo)
		if err != nil {
			return err
		}
	}
	return nil
}

// runMigrationsGroupMode applies all pending migrations, wrapping in TX (for tx-based), or no-tx (for *.ntx.)
func runMigrationsGroupMode(
	ctx context.Context,
	db *sql.DB,
	repo history.MigrateHistoryRepository,
	pendingMigrations []MigrationFile,
	directionDo bool,
) error {
	groupEntry, err := ParseFilesGroupMode(pendingMigrations)
	if err != nil {
		return err
	}
	return migrateListOfFilesFn(ctx, db, groupEntry.Files, groupEntry.UseTX, repo, directionDo)
}
