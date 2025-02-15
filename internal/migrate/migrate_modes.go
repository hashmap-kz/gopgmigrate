package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"gopgmigrate/internal/modes"

	"gopgmigrate/internal/vers"

	"github.com/google/uuid"

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

	// prepare migration scripts
	var pendingMigrations []vers.MigrationFile
	if migCtx.DirectionDo {
		pendingMigrations, err = getMigrationsForApply(ctx, conn, migCtx.MigrationDir, repo)
		if err != nil {
			return err
		}
	} else {
		pendingMigrations, err = getMigrationsForUndo(ctx, conn, migCtx.MigrationDir, repo, migCtx.UndoCount)
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
		return runMigrationsMixedMode(ctx, conn, repo, pendingMigrations, migCtx.DirectionDo)
	} else if migCtx.MigrateMode == modes.ModePlain {
		return runMigrationsPlainMode(ctx, conn, repo, pendingMigrations, migCtx.DirectionDo)
	} else if migCtx.MigrateMode == modes.ModeGroup {
		return runMigrationsGroupMode(ctx, conn, repo, pendingMigrations, migCtx.DirectionDo)
	}
	return fmt.Errorf("unknown mode: %s", migCtx.MigrateMode)
}

// runMigrationsPlainMode applies both versioned and repeatable migrations
func runMigrationsPlainMode(
	ctx context.Context,
	db *sql.DB,
	repo history.MigrateHistoryRepository,
	pendingMigrations []vers.MigrationFile,
	directionDo bool,
) error {
	iterId := uuid.New()
	for _, elem := range pendingMigrations {
		err := migrateOneScript(ctx, db, elem, repo, directionDo, iterId)
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
	pendingMigrations []vers.MigrationFile,
	directionDo bool,
) error {
	groupEntries, err := modes.ParseFilesMixedMode(pendingMigrations)
	if err != nil {
		return err
	}
	iterId := uuid.New()
	for _, elem := range groupEntries {
		err := migrateListOfFilesFn(ctx, db, elem.Files, elem.UseTX, repo, directionDo, iterId)
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
	pendingMigrations []vers.MigrationFile,
	directionDo bool,
) error {
	groupEntry, err := modes.ParseFilesGroupMode(pendingMigrations)
	if err != nil {
		return err
	}
	return migrateListOfFilesFn(ctx, db, groupEntry.Files, groupEntry.UseTX, repo, directionDo, uuid.New())
}

// init repo, conn

// TODO: this should be an interface
// TODO: simplify, cleanup
func initRepo(ctx context.Context, migCtx RunMigrationCtx) (history.MigrateHistoryRepository, *sql.DB, error) {
	var err error
	var repo history.MigrateHistoryRepository
	var conn *sql.DB

	if parseConnStr(migCtx.ConnStr) == modes.DbmsVendorPostgresql {
		repo = impl.NewMigrateHistoryPostgresRepository(ctx, migCtx.HistoryTableName)
		conn, err = dbms.GetDatabaseConnectionPostgres(migCtx.ConnStr)
		if err != nil {
			return nil, nil, err
		}

		err = repo.CreateHistoryTable(ctx, conn)
		if err != nil {
			return nil, nil, err
		}

	} else if parseConnStr(migCtx.ConnStr) == modes.DbmsVendorClickhouse {
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
		return modes.DbmsVendorPostgresql
	}
	if strings.HasPrefix(str, "clickhouse://") {
		return modes.DbmsVendorClickhouse
	}
	return ""
}
