package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"gopgmigrate/internal/migrate_history/impl"

	"gopgmigrate/internal/migrate_history"
)

// RunMigrations applies both versioned and repeatable migrations in a single transaction
func RunMigrations(ctx context.Context, conn *sql.DB, localFiles []migrationFile) error {
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

	// TODO: simplify from here a LOT

	// repository, helper functions for history-handling
	// works inside the same transaction as the other migration scripts
	mhRepo := impl.NewMigrateHistoryPostgresRepository(ctx, conn, "public.migrate_history")
	err = mhRepo.CreateHistoryTable(ctx)
	if err != nil {
		return err
	}

	// check that all applied migrations are present in files list
	migrateHistory, err := mhRepo.ListAll(ctx)
	if err != nil {
		return err
	}
	appliedMigrations := makeAppliedHistory(migrateHistory)
	err = checkHistory(appliedMigrations, localFiles)
	if err != nil {
		return err
	}

	// migrate
	versionedMigrationsToApply, err := getVersionedMigrationsToApply(appliedMigrations, localFiles)
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

// migrateOneScript applies versioned migrations for versioned/data
func migrateOneScript(ctx context.Context, conn *sql.DB, file migrationFile, mhRepo migrate_history.MigrateHistoryRepository) (err error) {
	useTX := !strings.HasSuffix(file.base, "ntx.sql")

	var tx *sql.Tx
	if useTX {
		tx, err = conn.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	slog.Info("migration",
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
		err = mhRepo.SaveRepeatable(ctx, &migrate_history.MigrateHistoryVersionedCreateInput{
			MhVersion: file.vers,
			MhName:    file.base,
			MhHash:    file.hash,
		})
		if err != nil {
			return err
		}
	} else {
		err = mhRepo.SaveVersioned(ctx, &migrate_history.MigrateHistoryVersionedCreateInput{
			MhVersion: file.vers,
			MhName:    file.base,
			MhHash:    file.hash,
		})
		if err != nil {
			return err
		}
	}

	if useTX {
		err := tx.Commit()
		if err != nil {
			return err
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
