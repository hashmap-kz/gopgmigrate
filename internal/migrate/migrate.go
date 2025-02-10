package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"gopgmigrate/internal/history"
)

// RunMigrations applies both versioned and repeatable migrations
func RunMigrations(
	ctx context.Context,
	db *sql.DB,
	mhRepo history.MigrateHistoryRepository,
	pending []MigrationFile,
	directionDo bool,
) error {
	for _, f := range pending {
		err := migrateOneScript(ctx, db, f, mhRepo, directionDo)
		if err != nil {
			return err
		}
	}
	return nil
}

// migrateOneScript applies versioned migrations for versioned/data
func migrateOneScript(
	ctx context.Context,
	db *sql.DB,
	file MigrationFile,
	mhRepo history.MigrateHistoryRepository,
	directionDo bool,
) (err error) {
	useTX := !versionedMigrationRegexNtx.MatchString(file.Base)

	if useTX {
		// TRANSACTION

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		slog.Info("migration",
			slog.String("tx", "+"),
			slog.String("mode", getModeForLog(directionDo)),
			slog.String("type", getTypeForLog(file)),
			slog.String("name", file.Base),
		)

		// execute migration script
		_, err = tx.ExecContext(ctx, string(file.data))
		if err != nil {
			return fmt.Errorf("error applying migration %s: %v", file.Base, err)
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

		err = tx.Commit()
		if err != nil {
			return err
		}
		return nil
	}

	// NO TRANSACTION

	slog.Info("migration",
		slog.String("tx", "N"),
		slog.String("mode", getModeForLog(directionDo)),
		slog.String("type", getTypeForLog(file)),
		slog.String("name", file.Base),
	)

	// execute migration script
	_, err = db.ExecContext(ctx, string(file.data))
	if err != nil {
		return fmt.Errorf("error applying migration %s: %v", file.Base, err)
	}

	// write history
	if directionDo {
		// DO
		if isRepeatable(file) {
			err = mhRepo.SaveRepeatableNoTx(ctx, db, &history.MigrateHistoryCreateInput{
				MhVersion: file.Vers,
				MhName:    file.Base,
				MhHash:    file.hash,
			})
			if err != nil {
				return err
			}
		} else {
			err = mhRepo.SaveVersionedNoTx(ctx, db, &history.MigrateHistoryCreateInput{
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
		err := mhRepo.DeleteVersionNoTx(ctx, db, file.Base)
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
