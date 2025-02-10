package cmd

import (
	"context"
	"database/sql"
	"log/slog"
	"os"

	"gopgmigrate/internal/dbms"
	"gopgmigrate/internal/history"
	"gopgmigrate/internal/history/impl"
	"gopgmigrate/internal/migrate"

	"github.com/spf13/cobra"
)

const (
	dbmsVendorPostgres   = "postgresql"
	dbmsVendorClickhouse = "clickhouse"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Run:   runMigrations,
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}

func runMigrations(cmd *cobra.Command, args []string) {
	var err error

	ctx := context.Background()

	// get migration scripts
	files, err := migrate.GetFiles(cliOptions.dirName)
	if err != nil {
		slog.Error("collecting files error", slog.String("err", err.Error()))
		os.Exit(1)
	}

	// repository, helper functions for history-handling
	var repo history.MigrateHistoryRepository
	var conn *sql.DB
	if cliOptions.dbms == dbmsVendorPostgres {
		repo = impl.NewMigrateHistoryPostgresRepository(ctx, cliOptions.historyTableName)
		conn, err = dbms.GetDatabaseConnectionPostgres(cliOptions.connStr)
		if err != nil {
			slog.Error("database connection error", slog.String("err", err.Error()))
			os.Exit(1)
		}
	} else {
		slog.Error("unknown DBMS vendor", slog.String("name", cliOptions.dbms))
		os.Exit(1)
	}
	defer conn.Close()

	// run all migrations in a single transaction
	err = migrate.RunMigrations(ctx, conn, files, repo)
	if err != nil {
		slog.Error("migration error", slog.String("err", err.Error()))
		os.Exit(1)
	}

	slog.Info("migrations applied successfully")
}
