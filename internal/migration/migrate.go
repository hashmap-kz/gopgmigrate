package migration

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"gopgmigrate/internal/filter"

	"gopgmigrate/internal/mode"

	"gopgmigrate/internal/version"

	"gopgmigrate/internal/dbms"
	"gopgmigrate/pkg/logger"

	"gopgmigrate/internal/history"
)

type RunMigrationCtx struct {
	MigrateMode  string
	DirectionDo  bool
	MigrationDir string
	DryRun       bool

	ConnStr          string
	HistoryTableName string

	UndoCount int
}

func RunMigrations(
	ctx context.Context,
	migCtx RunMigrationCtx,
) error {
	var err error

	// init repository
	repo, conn, err := initRepo(ctx, migCtx)
	if err != nil {
		return err
	}
	slog.Info("conn", slog.String("status", "opened"))
	defer func(conn *sql.DB) {
		err := conn.Close()
		if err != nil {
			slog.Warn("conn", slog.String("status", err.Error()))
		} else {
			slog.Info("conn", slog.String("status", "closed"))
		}
	}(conn)

	// lock

	acquired, err := repo.AcquireMigrationLock(ctx, conn)
	if err != nil {
		slog.Error("unable to acquire lock", slog.String("err", err.Error()))
		return fmt.Errorf("cannot acquire lock: %v", err)
	}
	if !acquired {
		slog.Error("another migration process is running. exiting.")
		return fmt.Errorf("cannot acquire lock: %v", err)
	}
	slog.Info("lock", slog.String("status", "acquired"))
	defer func(ctx context.Context, conn *sql.DB) {
		err = repo.ReleaseMigrationLock(ctx, conn)
		if err != nil {
			slog.Warn("lock", slog.String("status", err.Error()))
		} else {
			slog.Info("lock", slog.String("status", "released"))
		}
	}(ctx, conn)

	// migration

	return runMigrations(ctx, migCtx, conn, repo)
}

func runMigrations(ctx context.Context,
	migCtx RunMigrationCtx,
	conn *sql.DB,
	repo history.MigrateHistoryRepository,
) error {
	var err error

	// prepare migration scripts

	var pendingMigrations []version.MigrationFile
	if migCtx.DirectionDo {
		pendingMigrations, err = filter.GetMigrationsForApply(ctx, conn, migCtx.MigrationDir, repo)
		if err != nil {
			return err
		}
	} else {
		pendingMigrations, err = filter.GetMigrationsForUndo(ctx, conn, migCtx.MigrationDir, repo, migCtx.UndoCount)
		if err != nil {
			return err
		}
	}

	if migCtx.DryRun {
		_ = logger.DisableLogging()
		printMigrationsInfo(migCtx.MigrateMode, pendingMigrations)
		return nil
	}

	// migration

	if migCtx.MigrateMode == mode.ModeMixed {
		groupEntries, err := mode.ParseFilesMixedMode(pendingMigrations)
		if err != nil {
			return err
		}
		return repo.RunMigrationsMixedMode(ctx, conn, groupEntries, migCtx.DirectionDo)
	} else if migCtx.MigrateMode == mode.ModePlain {
		return repo.RunMigrationsPlainMode(ctx, conn, pendingMigrations, migCtx.DirectionDo)
	} else if migCtx.MigrateMode == mode.ModeGroup {
		groupEntry, err := mode.ParseFilesGroupMode(pendingMigrations)
		if err != nil {
			return err
		}
		return repo.RunMigrationsGroupMode(ctx, conn, groupEntry, migCtx.DirectionDo)
	}

	return fmt.Errorf("unknown mode: %s", migCtx.MigrateMode)
}

// init repo, conn

// TODO: simplify, cleanup
func initRepo(ctx context.Context, migCtx RunMigrationCtx) (history.MigrateHistoryRepository, *sql.DB, error) {
	var err error
	var repo history.MigrateHistoryRepository
	var conn *sql.DB

	repo = history.NewMigrateHistoryPostgresRepository(ctx, migCtx.HistoryTableName)
	conn, err = dbms.GetDatabaseConnectionPostgres(migCtx.ConnStr)
	if err != nil {
		return nil, nil, err
	}

	err = repo.CreateHistoryTable(ctx, conn)
	if err != nil {
		return nil, nil, err
	}

	return repo, conn, nil
}
