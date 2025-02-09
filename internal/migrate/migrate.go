package migrate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"gopgmigrate/internal/migrate_history"
)

// RunMigrations applies both versioned and repeatable migrations in a single transaction
func RunMigrations(ctx context.Context, conn *pgx.Conn, files *MigrationCtx) error {
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
	mhRepo := migrate_history.NewMigrateHistoryRepository(ctx, conn)

	// check that all applied migrations are present in files list
	appliedNames, err := mhRepo.GetAppliedNames(ctx)
	if err != nil {
		return err
	}
	err = checkHistory(appliedNames, files)
	if err != nil {
		return err
	}

	// I) migrate versioned
	versionedMigrationsToApply, err := getVersionedMigrationsToApply(ctx, files.versioned, mhRepo)
	if err != nil {
		return err
	}
	for _, f := range versionedMigrationsToApply {
		err = migrateOneScript(ctx, conn, f, mhRepo, "v")
		if err != nil {
			return err
		}
	}

	// II) migrate repeatable
	repeatableMigrationsToApply, err := getRepeatableMigrationsToApply(ctx, files.repeatable, mhRepo)
	if err != nil {
		return err
	}
	for _, f := range repeatableMigrationsToApply {
		err = migrateOneScript(ctx, conn, f, mhRepo, "r")
		if err != nil {
			return err
		}
	}

	return nil
}

// migrateOneScript applies versioned migrations for versioned/data
func migrateOneScript(ctx context.Context, conn *pgx.Conn, file migrationFile, mhRepo migrate_history.MigrateHistoryRepository, mod string) (err error) {
	useTX := !strings.HasSuffix(file.base, "ntx.sql")

	var tx pgx.Tx
	if useTX {
		tx, err = conn.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
	}

	slog.Info("migration",
		slog.String("mode", "VER"),
		slog.String("name", file.base),
	)

	// execute migration script
	_, err = conn.Exec(ctx, string(file.data))
	if err != nil {
		return fmt.Errorf("error applying migration %s: %v", file.base, err)
	}

	// write history
	if mod == "r" {
		// update history (upsert)
		_, err = mhRepo.SaveOrUpdateRepeatable(ctx, &migrate_history.MigrateHistoryRepeatableCreateInput{
			MhName: file.base,
			MhHash: computeHash(file.data),
		})
		if err != nil {
			return err
		}
	} else {
		version, err := parseVersionDo(file.base)
		if err != nil {
			return err
		}
		_, err = mhRepo.SaveVersioned(ctx, &migrate_history.MigrateHistoryVersionedCreateInput{
			MhVersion: version,
			MhName:    file.base,
			MhHash:    computeHash(file.data),
		})
		if err != nil {
			return err
		}
	}

	if useTX {
		err := tx.Commit(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func getVersionedMigrationsToApply(ctx context.Context, files []migrationFile, mhRepo migrate_history.MigrateHistoryRepository) ([]migrationFile, error) {
	applied, err := mhRepo.GetAppliedNames(ctx)
	if err != nil {
		return nil, err
	}

	var toApply []migrationFile

	for _, file := range files {
		// twice check a file given
		isVersioned := versionedMigrationRegexDo.MatchString(file.base)
		if !isVersioned {
			continue
		}
		// TODO: check that hash match
		// skip applied
		if applied[file.base] {
			continue
		}
		toApply = append(toApply, file)
	}
	return toApply, nil
}

func getRepeatableMigrationsToApply(ctx context.Context, files []migrationFile, mhRepo migrate_history.MigrateHistoryRepository) ([]migrationFile, error) {
	var toApply []migrationFile
	for _, file := range files {
		// twice check a file given
		isRepeatable := repeatableMigrationRegex.MatchString(file.base)
		if !isRepeatable {
			continue
		}
		newHash := computeHash(file.data)

		// Get stored hash
		var existingHash string
		migrateHistory, err := mhRepo.FindByName(ctx, file.base)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if migrateHistory != nil {
			existingHash = migrateHistory.MhHash
		}

		// Apply only if changed
		if existingHash != newHash {
			toApply = append(toApply, file)
		}
	}
	return toApply, nil
}
