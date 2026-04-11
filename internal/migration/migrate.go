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

	"gopgmigrate/pkg/logger"
)

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

type RunMigrationCtx struct {
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

	// 1) prepare

	pendingMigrations, err := preparePendingMigrations(ctx, migCtx, conn, repo)
	if err != nil {
		return err
	}

	if migCtx.DryRun {
		_ = logger.DisableLogging()
		printMigrationsInfo(pendingMigrations)
		return nil
	}

	// 2) apply

	return applyPendingMigrations(ctx, conn, repo, pendingMigrations, migCtx.DirectionDo)
}

func preparePendingMigrations(
	ctx context.Context,
	migCtx RunMigrationCtx,
	conn *sql.DB,
	repo history.MigrateHistoryRepository,
) ([]naming.MigrationFile, error) {
	var err error
	var pendingMigrations []naming.MigrationFile

	hist, err := repo.ListAll(ctx, conn)
	if err != nil {
		return nil, err
	}

	if migCtx.DirectionDo {
		pendingMigrations, err = filters.GetMigrationsForApply(ctx, hist, migCtx.MigrationDir)
		if err != nil {
			return nil, err
		}
	} else {
		pendingMigrations, err = filters.GetMigrationsForUndo(ctx, hist, migCtx.MigrationDir, migCtx.UndoCount)
		if err != nil {
			return nil, err
		}
	}
	return pendingMigrations, nil
}

// init repo, conn

func initRepo(ctx context.Context, migCtx RunMigrationCtx) (history.MigrateHistoryRepository, *sql.DB, error) {
	var err error
	var repo history.MigrateHistoryRepository
	var conn *sql.DB

	repo = history.NewMigrateHistoryPostgresRepository(ctx, migCtx.HistoryTableName)
	conn, err = newPgConnection(migCtx.ConnStr)
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
	mhRepo history.MigrateHistoryRepository,
	pendingMigrations []naming.MigrationFile,
	directionDo bool,
) error {
	for _, elem := range pendingMigrations {
		err := migrateOneScriptDecideTxNoTx(ctx, db, elem, mhRepo, directionDo)
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
	mhRepo history.MigrateHistoryRepository,
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
		err = migrateOneScriptFn(ctx, tx, script, file, mhRepo, directionDo, "+")
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
	return migrateOneScriptFn(ctx, db, script, file, mhRepo, directionDo, "N")
}

func migrateOneScriptFn(
	ctx context.Context,
	tx history.Transaction,
	script []string,
	file naming.MigrationFile,
	mhRepo history.MigrateHistoryRepository,
	directionDo bool,
	txLogNote string,
) (err error) {
	slog.Info("migration",
		slog.String("tx", txLogNote),
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

	// write history
	if directionDo {
		// DO
		if naming.IsRepeatable(file) {
			err = mhRepo.SaveRepeatable(ctx, tx, &history.MigrateHistoryCreateInput{
				Version: file.Vers,
				Name:    file.Base,
				Hash:    file.Hash,
			})
			if err != nil {
				return err
			}
		} else {
			err = mhRepo.SaveVersioned(ctx, tx, &history.MigrateHistoryCreateInput{
				Version: file.Vers,
				Name:    file.Base,
				Hash:    file.Hash,
			})
			if err != nil {
				return err
			}
		}
	} else {
		// UNDO
		err := mhRepo.DeleteVersion(ctx, tx, file.Vers)
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
