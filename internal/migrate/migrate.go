package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"gopgmigrate/internal/migrate_history"
)

// RunMigrations applies both versioned and repeatable migrations in a single transaction
func RunMigrations(ctx context.Context, conn *sql.DB, localFiles []migrationFile, mhRepo migrate_history.MigrateHistoryRepository) error {
	var err error

	// Acquire advisory lock
	acquired, err := acquireMigrationLock(ctx, conn)
	if err != nil {
		return err
	}
	if !acquired {
		fmt.Println("Another migration process is running. Exiting.")
		return nil
	}
	defer releaseMigrationLock(ctx, conn)

	// migrate
	versionedMigrationsToApply, err := getMigrations(ctx, conn, localFiles, mhRepo)
	if err != nil {
		return err
	}
	for _, f := range versionedMigrationsToApply {
		err = migrateOneScript(ctx, conn, f, mhRepo)
		if err != nil {
			return err
		}
	}

	return nil
}

func getMigrations(ctx context.Context, conn *sql.DB, localFiles []migrationFile, mhRepo migrate_history.MigrateHistoryRepository) ([]migrationFile, error) {
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

	appliedMigrations := makeAppliedHistory(migrateHistory)
	err = checkHistory(appliedMigrations, localFiles)
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
func migrateOneScript(ctx context.Context, conn *sql.DB, file migrationFile, mhRepo migrate_history.MigrateHistoryRepository) (err error) {
	useTX := !strings.HasSuffix(file.base, "ntx.sql")

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
			slog.String("name", file.base),
		)

		// execute migration script
		_, err = tx.ExecContext(ctx, string(file.data))
		if err != nil {
			return fmt.Errorf("error applying migration %s: %v", file.base, err)
		}

		// write history
		if isRepeatable(file) {
			err = mhRepo.SaveRepeatable(ctx, tx, &migrate_history.MigrateHistoryVersionedCreateInput{
				MhVersion: file.vers,
				MhName:    file.base,
				MhHash:    file.hash,
			})
			if err != nil {
				return err
			}
		} else {
			err = mhRepo.SaveVersioned(ctx, tx, &migrate_history.MigrateHistoryVersionedCreateInput{
				MhVersion: file.vers,
				MhName:    file.base,
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

	} else {
		// NO TRANSACTION

		slog.Info("migration",
			slog.String("TX", "N"),
			slog.String("mode", getModeForLog(file)),
			slog.String("name", file.base),
		)

		// execute migration script
		_, err = conn.ExecContext(ctx, string(file.data))
		if err != nil {
			return fmt.Errorf("error applying migration %s: %v", file.base, err)
		}

		// write history
		if isRepeatable(file) {
			err = mhRepo.SaveRepeatableNoTx(ctx, conn, &migrate_history.MigrateHistoryVersionedCreateInput{
				MhVersion: file.vers,
				MhName:    file.base,
				MhHash:    file.hash,
			})
			if err != nil {
				return err
			}
		} else {
			err = mhRepo.SaveVersionedNoTx(ctx, conn, &migrate_history.MigrateHistoryVersionedCreateInput{
				MhVersion: file.vers,
				MhName:    file.base,
				MhHash:    file.hash,
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func getModeForLog(file migrationFile) string {
	if isRepeatable(file) {
		return "REP"
	}
	return "VER"
}

func makeAppliedHistory(hist []migrate_history.MigrateHistory) AppliedHistory {
	r := AppliedHistory{}
	for _, elem := range hist {
		r[elem.MhName] = AppliedHistoryItem{
			MhName: elem.MhName,
			MhHash: elem.MhHash,
		}
	}
	return r
}
