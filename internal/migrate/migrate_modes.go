package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	"gopgmigrate/pkg/logger"

	"gopgmigrate/internal/history"
)

type RunMigrationCtx struct {
	MigrateMode  string
	DB           *sql.DB
	Repo         history.MigrateHistoryRepository
	DirectionDo  bool
	MigrationDir string
	DryRun       bool
}

func RunMigrations(
	ctx context.Context,
	migCtx RunMigrationCtx,
) error {
	// lock

	acquired, err := migCtx.Repo.AcquireMigrationLock(ctx, migCtx.DB)
	if err != nil {
		slog.Error("unable to acquire lock", slog.String("err", err.Error()))
		os.Exit(1)
	}
	if !acquired {
		slog.Error("another migration process is running. exiting.")
		os.Exit(1)
	}
	slog.Info("lock", slog.String("status", "acquired:true"))
	defer func(ctx context.Context, conn *sql.DB) {
		err = migCtx.Repo.ReleaseMigrationLock(ctx, conn)
		if err != nil {
			slog.Warn("lock", slog.String("status", err.Error()))
		} else {
			slog.Info("lock", slog.String("status", "released:true"))
		}
	}(ctx, migCtx.DB)

	// prepare

	pendingMigrations, err := getPendingMigrations(ctx, migCtx.DB, migCtx.MigrationDir, migCtx.Repo)
	if err != nil {
		return err
	}

	if migCtx.DryRun {
		_ = logger.DisableLogging()
		printMigrationsInfo(migCtx.MigrateMode, pendingMigrations)
		return nil
	}

	// migrate

	if migCtx.MigrateMode == ModeMixed {
		return runMigrationsMixedMode(ctx, migCtx.DB, migCtx.Repo, pendingMigrations, migCtx.DirectionDo)
	} else if migCtx.MigrateMode == ModePlain {
		return runMigrationsPlainMode(ctx, migCtx.DB, migCtx.Repo, pendingMigrations, migCtx.DirectionDo)
	} else if migCtx.MigrateMode == ModeGroup {
		return runMigrationsGroupMode(ctx, migCtx.DB, migCtx.Repo, pendingMigrations, migCtx.DirectionDo)
	}
	return fmt.Errorf("unknown mode: %s", migCtx.MigrateMode)
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
