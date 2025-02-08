package migrate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"gopgmigrate/internal/migrate_history"
)

// RunMigrations applies both versioned and repeatable migrations in a single transaction
func RunMigrations(conn *pgx.Conn, files *migrationCtx) error {
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

	repo := migrate_history.NewMigrateHistoryRepository(ctx, conn)

	// I) migrate schema
	err = migrateSchemaData(ctx, tx, migrationParams{
		mode:  schemaDirName,
		files: files.schema,
	}, repo)
	if err != nil {
		return err
	}

	// II) migrate repeatable
	err = migrateRepeatable(ctx, tx, migrationParams{
		mode:  repeatableDirName,
		files: files.repeatable,
	}, repo)
	if err != nil {
		return err
	}

	// III) migrate data
	err = migrateSchemaData(ctx, tx, migrationParams{
		mode:  dataDirName,
		files: files.data,
	}, repo)
	if err != nil {
		return err
	}

	// Commit transaction if everything succeeds
	return tx.Commit(ctx)
}

// migrateSchemaData applies versioned migrations for schema/data
func migrateSchemaData(ctx context.Context, tx pgx.Tx, mp migrationParams, mhRepo migrate_history.MigrateHistoryRepository) error {
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

		sql, err := os.ReadFile(file.path)
		if err != nil {
			return err
		}
		slog.Info("migration",
			slog.String("mode", "VER"),
			slog.String("path", file.base),
		)

		// execute migration script
		_, err = tx.Exec(ctx, string(sql))
		if err != nil {
			return fmt.Errorf("error applying migration %s: %v", file, err)
		}

		// write history
		_, err = mhRepo.Save(ctx, &migrate_history.MigrateHistoryCreateInput{
			MhVersion: &version,
			MhMode:    mp.mode,
			MhName:    file.base,
			MhHash:    computeHash(sql),
		})
		if err != nil {
			return err
		}

	}

	return nil
}

// migrateRepeatable applies repeatable migrations
func migrateRepeatable(ctx context.Context, tx pgx.Tx, mp migrationParams, mhRepo migrate_history.MigrateHistoryRepository) error {
	for _, file := range mp.files {
		sql, err := os.ReadFile(file.path)
		if err != nil {
			return err
		}
		newHash := computeHash(sql)
		name := file.base

		// Get stored hash
		var existingHash string
		migrateHistory, err := mhRepo.FindByNameMode(ctx, migrate_history.MigrateHistorySearchNameMode{
			MhName: name,
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
				slog.String("path", file.base),
			)

			_, err = tx.Exec(ctx, string(sql))
			if err != nil {
				return fmt.Errorf("error applying repeatable migration %s: %v", file, err)
			}

			// upsert
			if migrateHistory == nil {
				_, err := mhRepo.Save(ctx, &migrate_history.MigrateHistoryCreateInput{
					MhMode: repeatableDirName,
					MhName: name,
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
