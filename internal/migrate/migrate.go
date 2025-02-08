package migrate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

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

	// I) migrate schema
	err = migrateSchemaData(ctx, conn, migrationParams{
		mode:  schemaDirName,
		files: files.schema,
	}, mhRepo)
	if err != nil {
		return err
	}

	// II) migrate repeatable
	err = migrateRepeatable(ctx, conn, migrationParams{
		mode:  repeatableDirName,
		files: files.repeatable,
	}, mhRepo)
	if err != nil {
		return err
	}

	// III) migrate data
	err = migrateSchemaData(ctx, conn, migrationParams{
		mode:  dataDirName,
		files: files.data,
	}, mhRepo)
	if err != nil {
		return err
	}

	// Commit transaction if everything succeeds
	return tx.Commit(ctx)
}

// migrateSchemaData applies versioned migrations for schema/data
func migrateSchemaData(ctx context.Context, conn *pgx.Conn, mp migrationParams, mhRepo migrate_history.MigrateHistoryRepository) error {
	all, err := mhRepo.FindAllByMode(ctx, mp.mode)
	if err != nil {
		return err
	}
	applied := map[int64]bool{}
	for _, e := range all {
		if e.MhVersion == nil {
			return fmt.Errorf("unexpected nil version for applied migration: %s/%s", e.MhMode, e.MhName)
		}
		applied[*e.MhVersion] = true
	}

	for _, file := range mp.files {
		isVersioned := versionedMigrationRegexDo.MatchString(file.base)
		if !isVersioned {
			if !versionedMigrationRegexUndo.MatchString(file.base) {
				slog.Warn("skipped",
					slog.String("path", filepath.ToSlash(file.path)),
				)
			}
			continue
		}

		versionStr := strings.Split(filepath.Base(file.base), "-")[0]
		version, err := strconv.ParseInt(versionStr, 10, 64)
		if err != nil {
			return err
		}

		if applied[version] {
			continue
		}

		slog.Info("migration",
			slog.String("mode", "VER"),
			slog.String("name", file.base),
		)

		// execute migration script
		_, err = conn.Exec(ctx, string(file.data))
		if err != nil {
			return fmt.Errorf("error applying migration %s: %v", file, err)
		}

		// write history
		_, err = mhRepo.Save(ctx, &migrate_history.MigrateHistoryCreateInput{
			MhVersion: &version,
			MhMode:    mp.mode,
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
		newHash := computeHash(file.data)

		// Get stored hash
		var existingHash string
		migrateHistory, err := mhRepo.FindByNameMode(ctx, migrate_history.MigrateHistorySearchNameMode{
			MhName: file.base,
			MhMode: repeatableDirName,
		})
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
				return fmt.Errorf("error applying repeatable migration %s: %v", file, err)
			}

			// update history (upsert)
			if migrateHistory == nil {
				_, err := mhRepo.Save(ctx, &migrate_history.MigrateHistoryCreateInput{
					MhMode: repeatableDirName,
					MhName: file.base,
					MhHash: newHash,
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
