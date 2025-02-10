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
	conn *sql.DB,
	mhRepo history.MigrateHistoryRepository,
	pending []MigrationFile,
) error {
	for _, f := range pending {
		err := migrateOneScript(ctx, conn, f, mhRepo)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetPendingMigrations(
	ctx context.Context,
	conn *sql.DB,
	localFiles []MigrationFile,
	mhRepo history.MigrateHistoryRepository,
) ([]MigrationFile, error) {
	var err error

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	err = mhRepo.CreateHistoryTable(ctx, tx)
	if err != nil {
		return nil, err
	}

	// check that all applied migrations are present in files list
	migrateHistory, err := mhRepo.ListAll(ctx, tx)
	if err != nil {
		return nil, err
	}

	appliedMigrations := createAppliedHistoryIndex(migrateHistory)
	err = checkAppliedHistoryWithLocalFiles(appliedMigrations, localFiles)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return getVersionedMigrationsToApply(appliedMigrations, localFiles)
}

// migrateOneScript applies versioned migrations for versioned/data
func migrateOneScript(ctx context.Context, conn *sql.DB, file MigrationFile, mhRepo history.MigrateHistoryRepository) (err error) {
	useTX := !versionedMigrationRegexNtx.MatchString(file.Base)

	if useTX {
		// TRANSACTION

		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		slog.Info("migration",
			slog.String("TX", "Y"),
			slog.String("mode", getModeForLog(file)),
			slog.String("name", file.Base),
		)

		// execute migration script
		_, err = tx.ExecContext(ctx, string(file.data))
		if err != nil {
			return fmt.Errorf("error applying migration %s: %v", file.Base, err)
		}

		// write history
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

		err = tx.Commit()
		if err != nil {
			return err
		}
		return nil
	}

	// NO TRANSACTION

	slog.Info("migration",
		slog.String("TX", "N"),
		slog.String("mode", getModeForLog(file)),
		slog.String("name", file.Base),
	)

	// execute migration script
	_, err = conn.ExecContext(ctx, string(file.data))
	if err != nil {
		return fmt.Errorf("error applying migration %s: %v", file.Base, err)
	}

	// write history
	if isRepeatable(file) {
		err = mhRepo.SaveRepeatableNoTx(ctx, conn, &history.MigrateHistoryCreateInput{
			MhVersion: file.Vers,
			MhName:    file.Base,
			MhHash:    file.hash,
		})
		if err != nil {
			return err
		}
	} else {
		err = mhRepo.SaveVersionedNoTx(ctx, conn, &history.MigrateHistoryCreateInput{
			MhVersion: file.Vers,
			MhName:    file.Base,
			MhHash:    file.hash,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func getModeForLog(file MigrationFile) string {
	if isRepeatable(file) {
		return "REP"
	}
	return "VER"
}

func createAppliedHistoryIndex(hist []history.MigrateHistory) AppliedHistory {
	r := AppliedHistory{}
	for _, elem := range hist {
		r[elem.MhName] = AppliedHistoryItem{
			MhName: elem.MhName,
			MhHash: elem.MhHash,
		}
	}
	return r
}
