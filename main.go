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
	"strings"

	"github.com/jackc/pgx/v5"
)

// Migration Lock Key (must be unique per application)
const migrationLockKey = 123456

var versionedMigrationRegex = regexp.MustCompile(`^\d{7}-.*\.sql$`)

type MigrationFile struct {
	path string
	base string
	dir  string
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
		create table if not exists schema_migrations (
			id 			serial primary key,
			version 	varchar(8) unique not null,
			name 		text unique not null,
			hash 		text not null,
			applied_at 	timestamp default now()
		);
		create table if not exists schema_migrations_repeatable (
			id 			serial primary key,
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

// getFiles walks given directory recursively, sort result by basename
func getFiles(folder string) ([]MigrationFile, error) {
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

	// Run versioned migrations
	applied, err := GetAppliedMigrations(tx)
	if err != nil {
		return err
	}

	files, err := getFiles(folder)
	if err != nil {
		return err
	}

	for _, file := range files {
		isVersioned := versionedMigrationRegex.MatchString(file.base)
		isRepeatable := !isVersioned

		// Handle repeatable migrations
		if isRepeatable {
			sql, err := os.ReadFile(file.path)
			if err != nil {
				return err
			}
			newHash := ComputeHash(sql)
			name := filepath.Base(file.path)

			// Get stored hash
			var existingHash string
			err = tx.QueryRow(ctx, "SELECT hash FROM schema_migrations_repeatable WHERE name = $1", name).Scan(&existingHash)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}

			// Apply only if changed
			if existingHash != newHash {
				slog.Info("applying repeatable migration", slog.String("path", file.path))

				_, err = tx.Exec(ctx, string(sql))
				if err != nil {
					return fmt.Errorf("error applying repeatable migration %s: %v", file, err)
				}
				_, err = tx.Exec(ctx, `
					INSERT INTO schema_migrations_repeatable (name, hash)
					VALUES ($1, $2)
					ON CONFLICT (name) DO UPDATE SET hash = $2, applied_at = NOW()`, name, newHash)
				if err != nil {
					return err
				}
			}
		}

		// Handle versioned migrations
		if isVersioned {
			versionStr := strings.Split(filepath.Base(file.base), "-")[0]

			if direction == "apply" && applied[versionStr] {
				continue
			}
			if direction == "revert" && !applied[versionStr] {
				continue
			}

			sql, err := os.ReadFile(file.path)
			if err != nil {
				return err
			}
			newHash := ComputeHash(sql)

			slog.Info("applying versioned migration", slog.String("path", file.path))
			_, err = tx.Exec(ctx, string(sql))
			if err != nil {
				return fmt.Errorf("error applying migration %s: %v", file, err)
			}

			// Update schema_migrations table
			if direction == "apply" {
				_, err = tx.Exec(ctx, "INSERT INTO schema_migrations (version, hash, name) VALUES ($1, $2, $3)",
					versionStr,
					newHash,
					file.base,
				)
			} else {
				_, err = tx.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", versionStr)
			}
			if err != nil {
				return err
			}
		}
	}

	// Commit transaction if everything succeeds
	return tx.Commit(ctx)
}

// Get applied migrations
func GetAppliedMigrations(tx pgx.Tx) (map[string]bool, error) {
	rows, err := tx.Query(context.Background(), "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	migrations := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		migrations[version] = true
	}
	return migrations, nil
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

	// Run all migrations in a single transaction
	err = RunMigrations(conn, filepath.Join("migrations", "dev"), direction)
	if err != nil {
		log.Fatal("Migration error:", err)
	}

	fmt.Println("Migrations applied successfully!")
}
