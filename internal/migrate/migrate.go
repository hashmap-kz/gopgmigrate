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
func RunMigrations(conn *pgx.Conn, folder string) error {
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

	files, err := getFiles(folder)
	if err != nil {
		return err
	}

	// I) migrate schema
	err = migrateSchemaData(ctx, tx, migrationParams{
		table:  "public.migrate_schema",
		folder: folder,
		files:  files.schema,
		mode:   "SCH",
	})
	if err != nil {
		return err
	}

	// II) migrate repeatable
	err = migrateRepeatable(ctx, tx, migrationParams{
		table:  "public.migrate_repeatable",
		folder: folder,
		files:  files.repeatable,
		mode:   "REP",
	})
	if err != nil {
		return err
	}

	// III) migrate data
	err = migrateSchemaData(ctx, tx, migrationParams{
		table:  "public.migrate_data",
		folder: folder,
		files:  files.data,
		mode:   "DAT",
	})
	if err != nil {
		return err
	}

	// Commit transaction if everything succeeds
	return tx.Commit(ctx)
}

// EnsureSchemaMigrationTables checks that migration tracking tables exist
func EnsureSchemaMigrationTables(conn *pgx.Conn) error {
	query := `
		create table if not exists public.migrate_schema (
			id 			serial primary key,
			version 	int unique not null,
			name 		text unique not null,
			hash 		text not null,
			applied_at 	timestamp default now()
		);
		create table if not exists public.migrate_repeatable (
			id 			serial primary key,
			name 		text unique not null,
			hash 		text not null,
			applied_at 	timestamp default now()
		);
		create table if not exists public.migrate_data (
			id 			serial primary key,
			version 	int unique not null,
			name 		text unique not null,
			hash 		text not null,
			applied_at 	timestamp default now()
		);
	`
	_, err := conn.Exec(context.Background(), query)
	return err
}

// migrateSchemaData applies versioned migrations for schema/data
func migrateSchemaData(ctx context.Context, tx pgx.Tx, mp migrationParams) error {
	applied, err := getAppliedMigrations(tx, mp.table)
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
			slog.String("mode", mp.mode),
			slog.String("path", file.base),
		)

		// execute migration script
		_, err = tx.Exec(ctx, string(sql))
		if err != nil {
			return fmt.Errorf("error applying migration %s: %v", file, err)
		}

		// update history
		_, err = tx.Exec(ctx, fmt.Sprintf("INSERT INTO %s (version, hash, name) VALUES ($1, $2, $3)", mp.table),
			version,
			newHash,
			name,
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
		err = tx.QueryRow(ctx, fmt.Sprintf("SELECT hash FROM %s WHERE name = $1", mp.table), name).Scan(&existingHash)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		// Apply only if changed
		if existingHash != newHash {
			slog.Info("migration",
				slog.String("mode", mp.mode),
				slog.String("path", file.base),
			)

			_, err = tx.Exec(ctx, string(sql))
			if err != nil {
				return fmt.Errorf("error applying repeatable migration %s: %v", file, err)
			}
			_, err = tx.Exec(ctx, fmt.Sprintf(`
					INSERT INTO %s (name, hash)
					VALUES ($1, $2)
					ON CONFLICT (name) DO UPDATE SET hash = $2, applied_at = NOW()`, mp.table),
				name,
				newHash,
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Get applied migrations for a given history-table
func getAppliedMigrations(tx pgx.Tx, table string) (map[int]bool, error) {
	rows, err := tx.Query(context.Background(), fmt.Sprintf("SELECT version FROM %s where version is not null", table))
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
