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
)

// RunMigrations applies both versioned and repeatable migrations in a single transaction
func RunMigrations(conn *pgx.Conn, files *migrationCtx) error {
	var err error

	ctx := context.Background()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback(ctx) // Only rollback if error occurs
		}
	}()

	// Acquire advisory lock
	acquired, err := acquireMigrationLock(tx)
	if err != nil {
		return err
	}
	if !acquired {
		fmt.Println("Another migration process is running. Exiting.")
		return nil
	}
	defer releaseMigrationLock(tx) // Release lock after transaction

	// I) migrate schema
	err = migrateSchemaData(ctx, tx, migrationParams{
		mode:  schemaDirName,
		files: files.schema,
	})
	if err != nil {
		return err
	}

	// II) migrate repeatable
	err = migrateRepeatable(ctx, tx, migrationParams{
		mode:  repeatableDirName,
		files: files.repeatable,
	})
	if err != nil {
		return err
	}

	// III) migrate data
	err = migrateSchemaData(ctx, tx, migrationParams{
		mode:  dataDirName,
		files: files.data,
	})
	if err != nil {
		return err
	}

	// Commit transaction if everything succeeds
	return tx.Commit(ctx)
}

// migrateSchemaData applies versioned migrations for schema/data
func migrateSchemaData(ctx context.Context, tx pgx.Tx, mp migrationParams) error {
	applied, err := getAppliedMigrations(tx, mp.mode)
	if err != nil {
		return err
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
		version, err := strconv.Atoi(versionStr)
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
		newHash := computeHash(sql)
		name := file.base

		slog.Info("migration",
			slog.String("mode", "VER"),
			slog.String("path", file.base),
		)

		// execute migration script
		_, err = tx.Exec(ctx, string(sql))
		if err != nil {
			return fmt.Errorf("error applying migration %s: %v", file, err)
		}

		// update history
		query := fmt.Sprintf("INSERT INTO %s (version, hash, name, mode) VALUES ($1, $2, $3, $4)", defaultHistoryTableName)
		_, err = tx.Exec(ctx, query,
			version,
			newHash,
			name,
			mp.mode,
		)
		if err != nil {
			return err
		}

	}

	return nil
}

// migrateRepeatable applies repeatable migrations
func migrateRepeatable(ctx context.Context, tx pgx.Tx, mp migrationParams) error {
	for _, file := range mp.files {
		sql, err := os.ReadFile(file.path)
		if err != nil {
			return err
		}
		newHash := computeHash(sql)
		name := file.base

		// Get stored hash
		var existingHash string
		err = tx.QueryRow(ctx, fmt.Sprintf("SELECT hash FROM %s WHERE name = $1 and mode = $2", defaultHistoryTableName),
			name,
			repeatableDirName,
		).Scan(&existingHash)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
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
			_, err = tx.Exec(ctx, fmt.Sprintf(`
					INSERT INTO %s (name, hash, mode)
					VALUES ($1, $2, $3)
					ON CONFLICT (name, mode) DO UPDATE SET hash = $2, applied_at = NOW()`, defaultHistoryTableName),
				name,
				newHash,
				mp.mode,
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Get applied migrations for a given history-table
func getAppliedMigrations(tx pgx.Tx, mode string) (map[int]bool, error) {
	query := fmt.Sprintf("SELECT version FROM %s where version is not null and mode = $1", defaultHistoryTableName)
	rows, err := tx.Query(context.Background(), query, mode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	migrations := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		migrations[version] = true
	}
	return migrations, nil
}
