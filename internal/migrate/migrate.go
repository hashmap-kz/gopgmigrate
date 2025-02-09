package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"gopgmigrate/internal/migrate_history/impl"

	"gopgmigrate/internal/migrate_history"
)

// RunMigrations applies both versioned and repeatable migrations in a single transaction
func RunMigrations(ctx context.Context, conn *sql.DB, files *MigrationCtx) error {
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
	appliedNames, err := mhRepo.ListAll(ctx)
	if err != nil {
		return err
	}
	err = checkHistory(makeMapFromEntities(appliedNames), files)
	if err != nil {
		return err
	}

	// I) migrate versioned
	versionedMigrationsToApply, err := getVersionedMigrationsToApply(files.versioned, appliedNames)
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
	repeatableMigrationsToApply, err := getRepeatableMigrationsToApply(files.repeatable, appliedNames)
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
func migrateOneScript(ctx context.Context, conn *sql.DB, file migrationFile, mhRepo migrate_history.MigrateHistoryRepository, mod string) (err error) {
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
		slog.String("mode", "VER"),
		slog.String("name", file.base),
	)

	// execute migration script
	_, err = conn.ExecContext(ctx, string(file.data))
	if err != nil {
		return fmt.Errorf("error applying migration %s: %v", file.base, err)
	}

	// write history
	if mod == "r" {
		err = mhRepo.SaveRepeatable(ctx, &migrate_history.MigrateHistoryRepeatableCreateInput{
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
		err = mhRepo.SaveVersioned(ctx, &migrate_history.MigrateHistoryVersionedCreateInput{
			MhVersion: version,
			MhName:    file.base,
			MhHash:    computeHash(file.data),
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

func getVersionedMigrationsToApply(files []migrationFile, hist []migrate_history.MigrateHistory) ([]migrationFile, error) {
	var toApply []migrationFile
	for _, file := range files {
		// twice check a file given
		isVersioned := versionedMigrationRegexDo.MatchString(file.base)
		if !isVersioned {
			continue
		}

		// TODO: simplify, optimize, index-map
		// skip applied
		existing := findHist(file.base, hist)
		if existing != nil {
			if existing.MhHash != computeHash(file.data) {
				return nil, fmt.Errorf("hash mismatch, check migration script: %s", filepath.ToSlash(file.path))
			}
			continue
		}

		toApply = append(toApply, file)
	}
	return toApply, nil
}

func getRepeatableMigrationsToApply(files []migrationFile, hist []migrate_history.MigrateHistory) ([]migrationFile, error) {
	var toApply []migrationFile
	for _, file := range files {
		// twice check a file given
		isRepeatable := repeatableMigrationRegex.MatchString(file.base)
		if !isRepeatable {
			continue
		}
		newHash := computeHash(file.data)

		// TODO: simplify, optimize, index-map
		// Get stored hash
		existingHash := findHash(file.base, hist)

		// Apply only if changed
		if existingHash != newHash {
			toApply = append(toApply, file)
		}
	}
	return toApply, nil
}

func findHist(base string, hist []migrate_history.MigrateHistory) *migrate_history.MigrateHistory {
	for _, elem := range hist {
		if elem.MhName == base {
			return &elem
		}
	}
	return nil
}

func findHash(base string, hist []migrate_history.MigrateHistory) string {
	for _, elem := range hist {
		if elem.MhName == base {
			return elem.MhHash
		}
	}
	return ""
}

func makeMapFromEntities(names []migrate_history.MigrateHistory) map[string]bool {
	r := map[string]bool{}
	for _, elem := range names {
		r[elem.MhName] = true
	}
	return r
}
