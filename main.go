package main

import (
	"context"
	"log"
	"log/slog"
	"path/filepath"

	"gopgmigrate/internal/migrate"
)

func main() {
	var err error
	ctx := context.Background()

	// TODO: these are parameters (envs/cli)
	connString := "postgres://postgres:postgres@localhost:5432/bookstore"
	migrationDirectory := filepath.Join("migrations", "dev")

	// check given directory
	err = migrate.CheckMigrationDirectory(migrationDirectory)
	if err != nil {
		log.Fatalf("migration directory error: %v", err)
	}

	// connect to the database
	conn, err := migrate.GetDatabaseConnection(ctx, connString)
	if err != nil {
		log.Fatalf("database connection error: %v", err)
	}
	defer conn.Close(context.Background())

	// prepare history tables
	err = migrate.EnsureSchemaMigrationTables(conn)
	if err != nil {
		log.Fatalf("history tables error: %v", err)
	}

	// run all migrations in a single transaction
	err = migrate.RunMigrations(conn, migrationDirectory)
	if err != nil {
		log.Fatalf("migration error: %v", err)
	}

	slog.Info("migrations applied successfully")
}
