package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"gopgmigrate/internal/migrate_history/impl"

	"gopgmigrate/internal/migrate"

	// TODO: drivers package (clickhouse, postgres)
	_ "github.com/jackc/pgx/v5"
)

func main() {
	var err error
	ctx := context.Background()

	// TODO: these are parameters (envs/cli)
	// init logger
	logger := initLogger("console", "debug")
	slog.SetDefault(logger)

	// TODO: these are parameters (envs/cli)
	connString := "postgres://postgres:postgres@localhost:5432/bookstore"
	migrationDirectory := filepath.Join("examples", "basic")

	// connect to the database
	conn, err := migrate.GetDatabaseConnection(connString)
	if err != nil {
		log.Fatalf("database connection error: %v", err)
	}
	defer conn.Close()

	// get migration scripts
	files, err := migrate.GetFiles(migrationDirectory)
	if err != nil {
		log.Fatalf("collecting files error: %v", err)
	}

	// TODO: this one should be init in a factory-method
	// repository, helper functions for history-handling
	mhRepo := impl.NewMigrateHistoryPostgresRepository(ctx, "public.migrate_history")

	// run all migrations in a single transaction
	err = migrate.RunMigrations(ctx, conn, files, mhRepo)
	if err != nil {
		log.Fatalf("migration error: %v", err)
	}

	slog.Info("migrations applied successfully")
}

func initLogger(enc, lvl string) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: getLoggerLevel(lvl),
	}
	var logger *slog.Logger
	if enc == "console" {
		logger = slog.New(slog.NewTextHandler(os.Stdout, opts))
	} else {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return logger
}

// For mapping config logger to app logger levels
var loggerLevelMap = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

func getLoggerLevel(lvl string) slog.Level {
	level, exist := loggerLevelMap[lvl]
	if !exist {
		return slog.LevelDebug
	}

	return level
}
