package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"gopgmigrate/internal/stmts"

	"gopgmigrate/internal/dbms"

	"gopgmigrate/internal/history"
)

func migrateOneScriptFn(
	ctx context.Context,
	tx dbms.Transaction,
	script []string,
	file MigrationFile,
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
	for _, stmt := range script {
		_, err = tx.ExecContext(ctx, stmt)
		if err != nil {
			return fmt.Errorf("error applying migration %s: %v", file.Base, err)
		}
	}

	// write history
	if directionDo {
		// DO
		if isRepeatable(file) {
			err = mhRepo.SaveRepeatable(ctx, tx, &history.MigrateHistoryCreateInput{
				MhVersion: file.Vers,
				MhName:    file.Base,
				MhHash:    file.hash,
			})
			if err != nil {
				return err
			}
		} else {
			err = mhRepo.SaveVersioned(ctx, tx, &history.MigrateHistoryCreateInput{
				MhVersion: file.Vers,
				MhName:    file.Base,
				MhHash:    file.hash,
			})
			if err != nil {
				return err
			}
		}
	} else {
		// UNDO
		err := mhRepo.DeleteVersion(ctx, tx, file.Base)
		if err != nil {
			return err
		}
	}

	return nil
}

func migrateListOfFilesInTxFn(
	ctx context.Context,
	db *sql.DB,
	files []MigrationFile,
	repo history.MigrateHistoryRepository,
	directionDo bool,
) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, file := range files {
		script := []string{string(file.data)}
		err = migrateOneScriptFn(ctx, tx, script, file, repo, directionDo, "+")
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

// non-transactional scripts are executed statement-by-statement
func migrateListOfFilesNoTxFn(
	ctx context.Context,
	db *sql.DB,
	files []MigrationFile,
	repo history.MigrateHistoryRepository,
	directionDo bool,
) (err error) {
	for _, file := range files {
		script, _ := stmts.SplitSQLStatements(string(file.data))
		err = migrateOneScriptFn(ctx, db, script, file, repo, directionDo, "N")
		if err != nil {
			return err
		}
	}
	return nil
}

func migrateListOfFilesFn(
	ctx context.Context,
	db *sql.DB,
	files []MigrationFile,
	useTx bool,
	repo history.MigrateHistoryRepository,
	directionDo bool,
) (err error) {
	if useTx {
		err = migrateListOfFilesInTxFn(ctx, db, files, repo, directionDo)
		if err != nil {
			return err
		}
		return nil
	}
	err = migrateListOfFilesNoTxFn(ctx, db, files, repo, directionDo)
	if err != nil {
		return err
	}
	return nil
}

// migrateOneScript applies a single script, TX/NO-TX (based on filename pattern)
func migrateOneScript(
	ctx context.Context,
	db *sql.DB,
	file MigrationFile,
	mhRepo history.MigrateHistoryRepository,
	directionDo bool,
) (err error) {
	// TRANSACTION

	if isTx(file) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		script := []string{string(file.data)}
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

	script, _ := stmts.SplitSQLStatements(string(file.data))
	return migrateOneScriptFn(ctx, db, script, file, mhRepo, directionDo, "N")
}

func getModeForLog(directionDo bool) string {
	if directionDo {
		return "do"
	}
	return "undo"
}

func getTypeForLog(file MigrationFile) string {
	if isRepeatable(file) {
		return "rep"
	}
	return "ver"
}
