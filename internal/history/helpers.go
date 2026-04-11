package history

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"gopgmigrate/internal/dbms"
	"gopgmigrate/internal/stmt"
	"gopgmigrate/internal/version"
)

// MigrateOneScriptDecideTxNoTx applies a single script, TX/NO-TX (based on filename pattern)
func MigrateOneScriptDecideTxNoTx(
	ctx context.Context,
	db *sql.DB,
	file version.MigrationFile,
	mhRepo MigrateHistoryRepository,
	directionDo bool,
	iterID uuid.UUID,
) (err error) {
	// TRANSACTION

	if version.IsTx(file) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		script := []string{string(file.Data)}
		err = migrateOneScriptFn(ctx, tx, script, file, mhRepo, directionDo, "+", iterID)
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
	return migrateOneScriptFn(ctx, db, script, file, mhRepo, directionDo, "N", iterID)
}

func migrateOneScriptFn(
	ctx context.Context,
	tx dbms.Transaction,
	script []string,
	file version.MigrationFile,
	mhRepo MigrateHistoryRepository,
	directionDo bool,
	txLogNote string,
	iterID uuid.UUID,
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
		if version.IsRepeatable(file) {
			err = mhRepo.SaveRepeatable(ctx, tx, &MigrateHistoryCreateInput{
				MhVersion: file.Vers,
				MhName:    file.Base,
				MhHash:    file.Hash,
				MhIterID:  iterID.String(),
			})
			if err != nil {
				return err
			}
		} else {
			err = mhRepo.SaveVersioned(ctx, tx, &MigrateHistoryCreateInput{
				MhVersion: file.Vers,
				MhName:    file.Base,
				MhHash:    file.Hash,
				MhIterID:  iterID.String(),
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

func getTypeForLog(file version.MigrationFile) string {
	if version.IsRepeatable(file) {
		return "rep"
	}
	return "ver"
}
