package migration

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"gopgmigrate/internal/filters"
	"gopgmigrate/internal/history"

	"gopgmigrate/internal/stmt"

	"gopgmigrate/internal/naming"
)

// public API

type ApplyOpts struct {
	MigrationDir     string
	DryRun           bool
	ConnStr          string
	HistoryTableName string
}

type RollbackOpts struct {
	MigrationDir     string
	DryRun           bool
	ConnStr          string
	HistoryTableName string
	UndoCount        int
}

func RunMigrationsUp(ctx context.Context, o *ApplyOpts) error {
	return runMigrationsEntryPoint(ctx, &runMigrationCtx{
		directionDo:      true,
		migrationDir:     o.MigrationDir,
		dryRun:           o.DryRun,
		connStr:          o.ConnStr,
		historyTableName: o.HistoryTableName,
	})
}

func RunMigrationsDown(ctx context.Context, o *RollbackOpts) error {
	return runMigrationsEntryPoint(ctx, &runMigrationCtx{
		directionDo:      false,
		migrationDir:     o.MigrationDir,
		dryRun:           o.DryRun,
		connStr:          o.ConnStr,
		historyTableName: o.HistoryTableName,
		undoCount:        o.UndoCount,
	})
}

// internal impl

type runMigrationCtx struct {
	directionDo bool
	// common flags
	migrationDir     string
	dryRun           bool
	connStr          string
	historyTableName string
	// undo related only
	undoCount int
}

func runMigrationsEntryPoint(
	ctx context.Context,
	migCtx *runMigrationCtx,
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

	// prepare

	pendingMigrations, err := preparePendingMigrations(ctx, migCtx, conn, repo)
	if err != nil {
		return err
	}

	if migCtx.dryRun {
		printMigrationsInfo(pendingMigrations)
		return nil
	}

	// apply

	return applyPendingMigrations(ctx, conn, repo, pendingMigrations, migCtx.directionDo)
}

func preparePendingMigrations(
	ctx context.Context,
	migCtx *runMigrationCtx,
	conn *sql.DB,
	repo history.MigrateHistoryRepository,
) ([]naming.MigrationFile, error) {
	var err error
	var pendingMigrations []naming.MigrationFile

	hist, err := repo.ListAll(ctx, conn)
	if err != nil {
		return nil, err
	}

	if migCtx.directionDo {
		pendingMigrations, err = filters.GetMigrationsForApply(ctx, hist, migCtx.migrationDir)
		if err != nil {
			return nil, err
		}
	} else {
		pendingMigrations, err = filters.GetMigrationsForUndo(ctx, hist, migCtx.migrationDir, migCtx.undoCount)
		if err != nil {
			return nil, err
		}
	}
	return pendingMigrations, nil
}

// init repo, conn

func initRepo(ctx context.Context, migCtx *runMigrationCtx) (history.MigrateHistoryRepository, *sql.DB, error) {
	var err error
	var repo history.MigrateHistoryRepository
	var conn *sql.DB

	repo = history.NewMigrateHistoryPostgresRepository(ctx, migCtx.historyTableName)
	conn, err = newPgConnection(migCtx.connStr)
	if err != nil {
		return nil, nil, err
	}

	err = repo.CreateHistoryTable(ctx, conn)
	if err != nil {
		return nil, nil, err
	}

	return repo, conn, nil
}

// apply migrations runner

func applyPendingMigrations(
	ctx context.Context,
	db *sql.DB,
	repo history.MigrateHistoryRepository,
	pendingMigrations []naming.MigrationFile,
	directionDo bool,
) error {
	for _, elem := range pendingMigrations {
		err := migrateOneScriptDecideTxNoTx(ctx, db, elem, repo, directionDo)
		if err != nil {
			return err
		}
	}
	return nil
}

// migrateOneScriptDecideTxNoTx applies a single script, TX/NO-TX (based on filename pattern)
func migrateOneScriptDecideTxNoTx(
	ctx context.Context,
	db *sql.DB,
	file naming.MigrationFile,
	repo history.MigrateHistoryRepository,
	directionDo bool,
) (err error) {
	// TRANSACTION

	if naming.IsTx(file) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		script := []string{string(file.Data)}
		err = migrateOneScriptFn(ctx, tx, script, file, repo, directionDo)
		if err != nil {
			return err
		}

		err = tx.Commit()
		if err != nil {
			return err
		}
		return nil
	}

	// NO TRANSACTION

	script, _ := stmt.SplitSQLStatements(string(file.Data))
	return migrateOneScriptFn(ctx, db, script, file, repo, directionDo)
}

func migrateOneScriptFn(
	ctx context.Context,
	tx history.Transaction,
	script []string,
	file naming.MigrationFile,
	repo history.MigrateHistoryRepository,
	directionDo bool,
) (err error) {
	slog.Info("migration",
		slog.String("direction", getModeForLog(directionDo)),
		slog.String("type", getTypeForLog(file)),
		slog.String("name", file.Base),
	)

	// execute migration script
	for _, scriptStmt := range script {
		// skip empty
		if strings.TrimSpace(scriptStmt) == "" {
			continue
		}
		_, err = tx.ExecContext(ctx, scriptStmt)
		if err != nil {
			return fmt.Errorf("error applying migration %s: %v", file.Base, err)
		}
	}

	// prepare history entry
	historyCreateInput := &history.MigrateHistoryCreateInput{
		Version: file.Vers,
		Name:    file.Base,
		Hash:    file.Hash,
	}

	// write history
	if directionDo {
		// DO
		if naming.IsRepeatable(file) {
			err = repo.SaveRepeatable(ctx, tx, historyCreateInput)
			if err != nil {
				return err
			}
		} else {
			err = repo.SaveVersioned(ctx, tx, historyCreateInput)
			if err != nil {
				return err
			}
		}
	} else {
		// UNDO
		err := repo.DeleteVersion(ctx, tx, file.Vers)
		if err != nil {
			return err
		}
	}

	return nil
}

func getModeForLog(directionDo bool) string {
	if directionDo {
		return "do"
	}
	return "undo"
}

func getTypeForLog(file naming.MigrationFile) string {
	if naming.IsRepeatable(file) {
		return "rep"
	}
	return "ver"
}
