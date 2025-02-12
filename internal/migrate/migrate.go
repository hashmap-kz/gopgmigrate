package migrate

import (
	"context"
	"fmt"
	"log/slog"

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
		slog.String("mode", getModeForLog(directionDo)),
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
