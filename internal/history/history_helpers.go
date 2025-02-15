package history

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"gopgmigrate/internal/dbms"
	"gopgmigrate/internal/stmts"
	"gopgmigrate/internal/vers"
)

func MigrateListOfFiles(
	ctx context.Context,
	db *sql.DB,
	files []vers.MigrationFile,
	useTx bool,
	repo MigrateHistoryRepository,
	directionDo bool,
	iterId uuid.UUID,
) (err error) {
	if useTx {
		err = migrateListOfFilesInTxFn(ctx, db, files, repo, directionDo, iterId)
		if err != nil {
			return err
		}
		return nil
	}
	err = MigrateListOfFilesNoTx(ctx, db, files, repo, directionDo, iterId)
	if err != nil {
		return err
	}
	return nil
}

// MigrateListOfFilesNoTx non-transactional, scripts are executed statement-by-statement
func MigrateListOfFilesNoTx(
	ctx context.Context,
	db *sql.DB,
	files []vers.MigrationFile,
	repo MigrateHistoryRepository,
	directionDo bool,
	iterId uuid.UUID,
) (err error) {
	for _, file := range files {
		script, _ := stmts.SplitSQLStatements(string(file.Data))
		err = migrateOneScriptFn(ctx, db, script, file, repo, directionDo, "N", iterId)
		if err != nil {
			return err
		}
	}
	return nil
}

// MigrateOneScriptDecideTxNoTx applies a single script, TX/NO-TX (based on filename pattern)
func MigrateOneScriptDecideTxNoTx(
	ctx context.Context,
	db *sql.DB,
	file vers.MigrationFile,
	mhRepo MigrateHistoryRepository,
	directionDo bool,
	iterId uuid.UUID,
) (err error) {
	// TRANSACTION

	if vers.IsTx(file) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		script := []string{string(file.Data)}
		err = migrateOneScriptFn(ctx, tx, script, file, mhRepo, directionDo, "+", iterId)
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

	script, _ := stmts.SplitSQLStatements(string(file.Data))
	return migrateOneScriptFn(ctx, db, script, file, mhRepo, directionDo, "N", iterId)
}

func migrateOneScriptFn(
	ctx context.Context,
	tx dbms.Transaction,
	script []string,
	file vers.MigrationFile,
	mhRepo MigrateHistoryRepository,
	directionDo bool,
	txLogNote string,
	iterId uuid.UUID,
) (err error) {
	slog.Info("migration",
		slog.String("tx", txLogNote),
		slog.String("direction", getModeForLog(directionDo)),
		slog.String("type", getTypeForLog(file)),
		slog.String("name", file.Base),
	)

	// execute migration script
	for _, stmt := range script {
		// skip empty
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		_, err = tx.ExecContext(ctx, stmt)
		if err != nil {
			return fmt.Errorf("error applying migration %s: %v", file.Base, err)
		}
	}

	// write history
	if directionDo {
		// DO
		if vers.IsRepeatable(file) {
			err = mhRepo.SaveRepeatable(ctx, tx, &MigrateHistoryCreateInput{
				MhVersion: file.Vers,
				MhName:    file.Base,
				MhHash:    file.Hash,
				MhIterID:  iterId.String(),
			})
			if err != nil {
				return err
			}
		} else {
			err = mhRepo.SaveVersioned(ctx, tx, &MigrateHistoryCreateInput{
				MhVersion: file.Vers,
				MhName:    file.Base,
				MhHash:    file.Hash,
				MhIterID:  iterId.String(),
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

func migrateListOfFilesInTxFn(
	ctx context.Context,
	db *sql.DB,
	files []vers.MigrationFile,
	repo MigrateHistoryRepository,
	directionDo bool,
	iterId uuid.UUID,
) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, file := range files {
		script := []string{string(file.Data)}
		err = migrateOneScriptFn(ctx, tx, script, file, repo, directionDo, "+", iterId)
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

func getModeForLog(directionDo bool) string {
	if directionDo {
		return "do"
	}
	return "undo"
}

func getTypeForLog(file vers.MigrationFile) string {
	if vers.IsRepeatable(file) {
		return "rep"
	}
	return "ver"
}
