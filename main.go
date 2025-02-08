package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"gopgmigrate/internal/migrate"
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

	// get migration scripts
	files, err := migrate.GetFiles(migrationDirectory)
	if err != nil {
		log.Fatalf("collecting files error: %v", err)
	}

	// TODO: repo
	//err = migrate.CheckHistory(conn, files)
	//if err != nil {
	//	log.Fatalf("migration history error: %v", err)
	//}

	// run all migrations in a single transaction
	err = migrate.RunMigrations(conn, files)
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
