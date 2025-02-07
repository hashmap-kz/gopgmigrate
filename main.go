package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Migration Lock Key (must be unique per application)
const migrationLockKey = 123456

const (
	schemaDirName     = "schema"
	repeatableDirName = "repeatable"
	dataDirName       = "data"
)

var (
	// example: 00003-users.do.sql
	versionedMigrationRegexDo = regexp.MustCompile(`^\d{5}-.*\.do\.sql$`)

	// example: 00003-users.undo.sql
	versionedMigrationRegexUndo = regexp.MustCompile(`^\d{5}-.*\.undo\.sql$`)
)

type migrationFile struct {
	path string
	base string
	dir  string
}

type migrationCtx struct {
	schema     []migrationFile
	repeatable []migrationFile
	data       []migrationFile
}

type migrationParams struct {
	table  string
	folder string
	files  []migrationFile
	mode   string // schema, data, repeatable: for logging only
}

// getDatabaseConnection initializes a PostgreSQL connection
func getDatabaseConnection(ctx context.Context, dbURL string) (*pgx.Conn, error) {
	return pgx.Connect(ctx, dbURL)
}

// acquireMigrationLock ensures only one migration process runs at a time
func acquireMigrationLock(tx pgx.Tx) (bool, error) {
	ctx := context.Background()
	var acquired bool
	err := tx.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", migrationLockKey).Scan(&acquired)
	return acquired, err
}

// releaseMigrationLock releases the advisory lock
func releaseMigrationLock(tx pgx.Tx) error {
	_, err := tx.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockKey)
	return err
}

// ensureSchemaMigrationTables checks that migration tracking tables exist
func ensureSchemaMigrationTables(conn *pgx.Conn) error {
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

// computeHash computes SHA256 hash of a file
func computeHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// getFilesInAPath walks path, collects all *.sql files
func getFilesInAPath(folder string) ([]migrationFile, error) {
	var files []migrationFile
	err := filepath.WalkDir(folder, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".sql") {
			files = append(files, migrationFile{
				path: path,
				base: filepath.Base(path),
				dir:  filepath.Dir(path),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Sort by base (Ascending)
	sort.Slice(files, func(i, j int) bool {
		return files[i].base < files[j].base
	})
	return files, nil
}

// getFiles walks given directory recursively, sort result by basename
func getFiles(folder string) (*migrationCtx, error) {
	schemaFiles, err := getFilesInAPath(filepath.Join(folder, schemaDirName))
	if err != nil {
		return nil, err
	}
	repeatableFiles, err := getFilesInAPath(filepath.Join(folder, repeatableDirName))
	if err != nil {
		return nil, err
	}
	dataFiles, err := getFilesInAPath(filepath.Join(folder, dataDirName))
	if err != nil {
		return nil, err
	}
	return &migrationCtx{
		schema:     schemaFiles,
		repeatable: repeatableFiles,
		data:       dataFiles,
	}, nil
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

// runMigrations applies both versioned and repeatable migrations in a single transaction
func runMigrations(conn *pgx.Conn, folder string) error {
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

// directoryExists checks that a given path exists and it's a directory
func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// checkMigrationDirectory checks that the migration-directory structure is conforming for all rules
func checkMigrationDirectory(folder string) error {
	requiredDirs := []string{schemaDirName, repeatableDirName, dataDirName}
	for _, dir := range requiredDirs {
		fullPath := filepath.Join(folder, dir)
		if !directoryExists(fullPath) {
			return fmt.Errorf("%s directory does not exist in: %s", dir, folder)
		}
	}
	return nil
}

func main() {
	ctx := context.Background()

	// Connect to the database
	conn, err := getDatabaseConnection(ctx, "postgres://postgres:postgres@localhost:5432/bookstore")
	if err != nil {
		log.Fatal("Database connection error:", err)
	}
	defer conn.Close(context.Background())

	err = ensureSchemaMigrationTables(conn)
	if err != nil {
		log.Fatal(err)
	}

	migrationDirectory := filepath.Join("migrations", "dev")
	err = checkMigrationDirectory(migrationDirectory)
	if err != nil {
		log.Fatal("Migration directory error:", err)
	}

	// Run all migrations in a single transaction
	err = runMigrations(conn, migrationDirectory)
	if err != nil {
		log.Fatal("Migration error:", err)
	}

	slog.Info("migrations applied successfully")
}
