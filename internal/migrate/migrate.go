package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"gopgmigrate/internal/filters"

	"gopgmigrate/internal/modes"

	"gopgmigrate/internal/vers"

	"gopgmigrate/internal/dbms"
	"gopgmigrate/internal/history/impl"

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

	// migrate

	return runMigrations(ctx, migCtx, conn, repo)
}

func runMigrations(ctx context.Context,
	migCtx RunMigrationCtx,
	conn *sql.DB,
	repo history.MigrateHistoryRepository,
) error {
	var err error

	// prepare migration scripts

	var pendingMigrations []vers.MigrationFile
	if migCtx.DirectionDo {
		pendingMigrations, err = filters.GetMigrationsForApply(ctx, conn, migCtx.MigrationDir, repo)
		if err != nil {
			return err
		}
	} else {
		pendingMigrations, err = filters.GetMigrationsForUndo(ctx, conn, migCtx.MigrationDir, repo, migCtx.UndoCount)
		if err != nil {
			return err
		}
	}

	if migCtx.DryRun {
		_ = logger.DisableLogging()
		printMigrationsInfo(migCtx.MigrateMode, pendingMigrations)
		return nil
	}

	// migrate

	if migCtx.MigrateMode == modes.ModeMixed {
		groupEntries, err := modes.ParseFilesMixedMode(pendingMigrations)
		if err != nil {
			return err
		}
		return repo.RunMigrationsMixedMode(ctx, conn, groupEntries, migCtx.DirectionDo)
	} else if migCtx.MigrateMode == modes.ModePlain {
		return repo.RunMigrationsPlainMode(ctx, conn, pendingMigrations, migCtx.DirectionDo)
	} else if migCtx.MigrateMode == modes.ModeGroup {
		groupEntry, err := modes.ParseFilesGroupMode(pendingMigrations)
		if err != nil {
			return err
		}
		return repo.RunMigrationsGroupMode(ctx, conn, groupEntry, migCtx.DirectionDo)
	}

	return fmt.Errorf("unknown mode: %s", migCtx.MigrateMode)
}

// init repo, conn

// TODO: this should be an interface
// TODO: simplify, cleanup
func initRepo(ctx context.Context, migCtx RunMigrationCtx) (history.MigrateHistoryRepository, *sql.DB, error) {
	var err error
	var repo history.MigrateHistoryRepository
	var conn *sql.DB

	if parseConnStr(migCtx.ConnStr) == dbms.VendorPostgresql {
		repo = impl.NewMigrateHistoryPostgresRepository(ctx, migCtx.HistoryTableName)
		conn, err = dbms.GetDatabaseConnectionPostgres(migCtx.ConnStr)
		if err != nil {
			return nil, nil, err
		}

		err = repo.CreateHistoryTable(ctx, conn)
		if err != nil {
			return nil, nil, err
		}

	} else if parseConnStr(migCtx.ConnStr) == dbms.VendorClickhouse {
		repo = impl.NewMigrateHistoryClickhouseRepository(ctx, migCtx.HistoryTableName)
		conn, err = dbms.GetDatabaseConnectionClickhouse(migCtx.ConnStr)
		if err != nil {
			return nil, nil, err
		}

		err = repo.CreateHistoryTable(ctx, conn)
		if err != nil {
			return nil, nil, err
		}

	} else {
		slog.Error("unknown DBMS vendor", slog.String("connStr", migCtx.ConnStr))
		return nil, nil, fmt.Errorf("unknown DBMS vendor for connStr: %s", migCtx.ConnStr)
	}

	return repo, conn, nil
}

func parseConnStr(str string) string {
	if strings.HasPrefix(str, "postgres://") {
		return dbms.VendorPostgresql
	}
	if strings.HasPrefix(str, "clickhouse://") {
		return dbms.VendorClickhouse
	}
	return ""
}
