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

// example: 00003-users.do.sql
var (
	versionedMigrationRegexDo   = regexp.MustCompile(`^\d{5}-.*\.do\.sql$`)
	versionedMigrationRegexUndo = regexp.MustCompile(`^\d{5}-.*\.undo\.sql$`)
)

type MigrationFile struct {
	path string
	base string
	dir  string
}

type Migration struct {
	schema     []MigrationFile
	repeatable []MigrationFile
	data       []MigrationFile
}

type MigrationParams struct {
	direction string
	table     string
	folder    string
	files     []MigrationFile
	mode      string // schema, data, repeatable: for logging only
}

// GetDatabaseConnection initializes a PostgreSQL connection
func GetDatabaseConnection(dbURL string) (*pgx.Conn, error) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	return conn, err
}

// AcquireMigrationLock ensures only one migration process runs at a time
func AcquireMigrationLock(tx pgx.Tx) (bool, error) {
	ctx := context.Background()
	var acquired bool
	err := tx.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", migrationLockKey).Scan(&acquired)
	return acquired, err
}

// ReleaseMigrationLock releases the advisory lock
func ReleaseMigrationLock(tx pgx.Tx) error {
	_, err := tx.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockKey)
	return err
}

// Ensure migration tracking tables exist
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

// Compute SHA256 hash of a file
func ComputeHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

func getFilesInAPath(folder string) ([]MigrationFile, error) {
	var files []MigrationFile
	err := filepath.WalkDir(folder, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".sql") {
			files = append(files, MigrationFile{
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
func getFiles(folder string) (*Migration, error) {
	schemaFiles, err := getFilesInAPath(filepath.Join(folder, "schema"))
	if err != nil {
		return nil, err
	}
	repeatableFiles, err := getFilesInAPath(filepath.Join(folder, "repeatable"))
	if err != nil {
		return nil, err
	}
	dataFiles, err := getFilesInAPath(filepath.Join(folder, "data"))
	if err != nil {
		return nil, err
	}
	return &Migration{
		schema:     schemaFiles,
		repeatable: repeatableFiles,
		data:       dataFiles,
	}, nil
}

func migrateSchemaData(ctx context.Context, tx pgx.Tx, mp MigrationParams) error {
	applied, err := GetAppliedMigrations(tx, mp.table)
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

		if mp.direction == "apply" && applied[version] {
			continue
		}
		if mp.direction == "revert" && !applied[version] {
			continue
		}

		sql, err := os.ReadFile(file.path)
		if err != nil {
			return err
		}
		newHash := ComputeHash(sql)
		name := file.base

		slog.Info("migration",
			slog.String("mode", mp.mode),
			slog.String("path", file.base),
		)

		_, err = tx.Exec(ctx, string(sql))
		if err != nil {
			return fmt.Errorf("error applying migration %s: %v", file, err)
		}

		if mp.direction == "apply" {
			_, err = tx.Exec(ctx, fmt.Sprintf("INSERT INTO %s (version, hash, name) VALUES ($1, $2, $3)", mp.table),
				version,
				newHash,
				name,
			)
		} else {
			_, err = tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE version = $1", mp.table), version)
		}
		if err != nil {
			return err
		}

	}

	return nil
}

func migrateRepeatable(ctx context.Context, tx pgx.Tx, mp MigrationParams) error {
	for _, file := range mp.files {
		sql, err := os.ReadFile(file.path)
		if err != nil {
			return err
		}
		newHash := ComputeHash(sql)
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

// RunMigrations applies both versioned and repeatable migrations in a single transaction
func RunMigrations(conn *pgx.Conn, folder, direction string) error {
	ctx := context.Background()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) // Ensures rollback if anything fails

	// Acquire advisory lock
	acquired, err := AcquireMigrationLock(tx)
	if err != nil {
		return err
	}
	if !acquired {
		fmt.Println("Another migration process is running. Exiting.")
		return nil
	}
	defer ReleaseMigrationLock(tx) // Release lock after transaction

	files, err := getFiles(folder)
	if err != nil {
		return err
	}

	// I) migrate schema
	err = migrateSchemaData(ctx, tx, MigrationParams{
		direction: direction,
		table:     "public.migrate_schema",
		folder:    folder,
		files:     files.schema,
		mode:      "SCH",
	})
	if err != nil {
		return err
	}

	// II) migrate repeatable
	err = migrateRepeatable(ctx, tx, MigrationParams{
		direction: direction,
		table:     "public.migrate_repeatable",
		folder:    folder,
		files:     files.repeatable,
		mode:      "REP",
	})
	if err != nil {
		return err
	}

	// III) migrate data
	err = migrateSchemaData(ctx, tx, MigrationParams{
		direction: direction,
		table:     "public.migrate_data",
		folder:    folder,
		files:     files.data,
		mode:      "DAT",
	})
	if err != nil {
		return err
	}

	// Commit transaction if everything succeeds
	return tx.Commit(ctx)
}

// Get applied migrations
func GetAppliedMigrations(tx pgx.Tx, table string) (map[int]bool, error) {
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

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		return false
	}
	return info.IsDir()
}

func checkFolder(folder string) error {
	schemaDir := filepath.Join(folder, "schema")
	if !directoryExists(schemaDir) {
		return fmt.Errorf("schema directory not exist in a given directory: %s", folder)
	}

	repeatableDir := filepath.Join(folder, "repeatable")
	if !directoryExists(repeatableDir) {
		return fmt.Errorf("repeatable directory not exist in a given directory: %s", folder)
	}

	dataDir := filepath.Join(folder, "data")
	if !directoryExists(dataDir) {
		return fmt.Errorf("data directory not exist in a given directory: %s", folder)
	}

	return nil
}

func main() {
	// Connect to the database
	conn, err := GetDatabaseConnection("postgres://postgres:postgres@localhost:5432/bookstore")
	if err != nil {
		log.Fatal("Database connection error:", err)
	}
	defer conn.Close(context.Background())

	err = EnsureSchemaMigrationTables(conn)
	if err != nil {
		log.Fatal(err)
	}

	//// Parse command-line arguments
	//if len(os.Args) < 2 {
	//	fmt.Println("Usage: go run main.go [up|down]")
	//	os.Exit(1)
	//}
	//direction := os.Args[1]

	direction := "apply"

	folder := filepath.Join("migrations", "dev")
	err = checkFolder(folder)
	if err != nil {
		log.Fatal("Migration directory error:", err)
	}

	// Run all migrations in a single transaction
	err = RunMigrations(conn, folder, direction)
	if err != nil {
		log.Fatal("Migration error:", err)
	}

	slog.Info("migrations applied successfully")
}
