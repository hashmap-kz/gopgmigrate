package migrate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"gopgmigrate/internal/migrate_history"
)

// RunMigrations applies both versioned and repeatable migrations in a single transaction
func RunMigrations(conn *pgx.Conn, files *MigrationCtx) error {
	var err error

	ctx := context.Background()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Acquire advisory lock
	acquired, err := acquireMigrationLock(tx)
	if err != nil {
		return err
	}
	if !acquired {
		fmt.Println("Another migration process is running. Exiting.")
		return nil
	}
	defer releaseMigrationLock(tx)

	// repository, helper functions for history-handling
	// works inside the same transaction as the other migration scripts
	mhRepo := migrate_history.NewMigrateHistoryRepository(ctx, conn)

	// check that all applied migrations are present in files list
	err = checkHistory(ctx, mhRepo, files)
	if err != nil {
		return err
	}

	// I) migrate versioned
	err = migrateVersioned(ctx, conn, migrationParams{
		files: files.versioned,
	}, mhRepo)
	if err != nil {
		return err
	}

	// II) migrate repeatable
	err = migrateRepeatable(ctx, conn, migrationParams{
		files: files.repeatable,
	}, mhRepo)
	if err != nil {
		return err
	}

	// Commit transaction if everything succeeds
	return tx.Commit(ctx)
}

// migrateVersioned applies versioned migrations for versioned/data
func migrateVersioned(ctx context.Context, conn *pgx.Conn, mp migrationParams, mhRepo migrate_history.MigrateHistoryRepository) error {
	applied, err := mhRepo.GetAppliedNames(ctx)
	if err != nil {
		return err
	}

	for _, file := range mp.files {
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
		version, err := parseVersionDo(file.base)
		if err != nil {
			return err
		}
		_, err = mhRepo.Save(ctx, &migrate_history.MigrateHistoryCreateInput{
			MhVersion: version,
			MhName:    file.base,
			MhHash:    computeHash(file.data),
		})
		if err != nil {
			return err
		}

	}

	return nil
}

// migrateRepeatable applies repeatable migrations
func migrateRepeatable(ctx context.Context, conn *pgx.Conn, mp migrationParams, mhRepo migrate_history.MigrateHistoryRepository) error {
	for _, file := range mp.files {
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
			return err
		}
		if migrateHistory != nil {
			existingHash = migrateHistory.MhHash
		}

		// Apply only if changed
		if existingHash != newHash {
			slog.Info("migration",
				slog.String("mode", "REP"),
				slog.String("name", file.base),
			)

			// execute script
			_, err = conn.Exec(ctx, string(file.data))
			if err != nil {
				return fmt.Errorf("error applying repeatable migration %s: %v", file.base, err)
			}

			// update history (upsert)
			if migrateHistory == nil {
				_, err := mhRepo.Save(ctx, &migrate_history.MigrateHistoryCreateInput{
					MhVersion: -1,
					MhName:    file.base,
					MhHash:    newHash,
				})
				if err != nil {
					return err
				}
			} else {
				_, err := mhRepo.UpdateByID(ctx, newHash, migrateHistory.ID)
				if err != nil {
					return err
				}
			}

		}
	}
	return nil
}

// history checker

func checkHistory(ctx context.Context, mhRepo migrate_history.MigrateHistoryRepository, files *MigrationCtx) error {
	appliedNames, err := mhRepo.GetAppliedNames(ctx)
	if err != nil {
		return err
	}

	all := files.repeatable
	all = append(all, files.versioned...)

	err = checkHistoryTableIsSyncedWithLocalFiles(appliedNames, all)
	if err != nil {
		return err
	}
	return nil
}

func checkHistoryTableIsSyncedWithLocalFiles(migrations map[string]bool, mf []migrationFile) error {
	for k := range migrations {
		if !found(k, mf) {
			return fmt.Errorf("detected applied migration not resolved locally: %s", k)
		}
	}
	return nil
}

func found(k string, mf []migrationFile) bool {
	for _, f := range mf {
		if k == f.base {
			return true
		}
	}
	return false
}
