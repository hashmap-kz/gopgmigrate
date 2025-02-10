package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"gopgmigrate/pkg/logger"

	"gopgmigrate/internal/history/impl"

	"gopgmigrate/internal/migrate"

	// TODO: drivers package (clickhouse, postgres)
	_ "github.com/jackc/pgx/v5"
)

func main() {
	var err error
	ctx := context.Background()

	// TODO: these are parameters (envs/cli)
	// init logger
	slog.SetDefault(logger.InitLogger("console", "debug"))

	// TODO: these are parameters (envs/cli)
	connString := "postgres://postgres:postgres@localhost:5432/bookstore"
	migrationDirectory := filepath.Join("examples", "basic")

	// connect to the database
	conn, err := migrate.GetDatabaseConnection(connString)
	if err != nil {
		slog.Error("database connection error", slog.String("err", err.Error()))
		os.Exit(1)
	}
	defer conn.Close()

	// get migration scripts
	files, err := migrate.GetFiles(migrationDirectory)
	if err != nil {
		slog.Error("collecting files error", slog.String("err", err.Error()))
		os.Exit(1)
	}

	// TODO: this one should be init in a factory-method
	// repository, helper functions for history-handling
	mhRepo := impl.NewMigrateHistoryPostgresRepository(ctx, "public.migrate_history")

	// run all migrations in a single transaction
	err = migrate.RunMigrations(ctx, conn, files, mhRepo)
	if err != nil {
		slog.Error("migration error", slog.String("err", err.Error()))
		os.Exit(1)
	}

	slog.Info("migrations applied successfully")
}
