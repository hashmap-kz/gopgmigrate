package main

import (
	"context"
	"database/sql"
	"fmt"
	"gopgmigrate/internal/dbms"
	"log/slog"
	"os"
	"strconv"

	"gopgmigrate/internal/history"
	"gopgmigrate/internal/history/impl"
	"gopgmigrate/internal/migrate"
	"gopgmigrate/pkg/logger"

	// TODO: drivers package (clickhouse, postgres)
	_ "github.com/jackc/pgx/v5"
	"github.com/spf13/cobra"
)

const (
	dbmsVendorPostgres   = "postgresql"
	dbmsVendorClickhouse = "clickhouse"
)

var cliOptions struct {
	dirName          string
	connStr          string
	config           string
	logEnc           string
	logLevel         string
	historyTableName string
	dbms             string
}

func RunMigrations(cmd *cobra.Command, args []string) {
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

func checkRequired() {
	if cliOptions.dirName == "" {
		cliOptions.dirName = os.Getenv("PGMIGRATE_DIRNAME")
		if cliOptions.dirName == "" {
			slog.Error("config", slog.String("cause", "dirname cannot be empty"))
			os.Exit(1)
		}
	}
	if cliOptions.connStr == "" {
		cliOptions.connStr = os.Getenv("PGMIGRATE_CONNSTR")
		if cliOptions.connStr == "" {
			slog.Error("config", slog.String("cause", "connstr cannot be empty"))
			os.Exit(1)
		}
	}
	// TODO: check given table name is conforming postgresql naming conventions (len, ascii, ident, etc)
	if cliOptions.historyTableName == "" {
		cliOptions.historyTableName = os.Getenv("PGMIGRATE_HISTORY_TABLE_NAME")
		if cliOptions.historyTableName == "" {
			slog.Error("config", slog.String("cause", "history-table-name cannot be empty"))
			os.Exit(1)
		}
	}
}

func addCoreConfigFlagsToCommand(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&cliOptions.config, "config", "c", "", "config path")
	cmd.Flags().StringVarP(&cliOptions.dirName, "dirname", "d", "", "migrations path (default is .)")
	cmd.Flags().StringVarP(&cliOptions.connStr, "connstr", "", "", "database connection string")
	cmd.Flags().StringVarP(&cliOptions.logEnc, "logEnc", "", "console", "log encoding: (console, json)")
	cmd.Flags().StringVarP(&cliOptions.logLevel, "logLevel", "", "debug", "log level: (debug, info, warn, error)")
	cmd.Flags().StringVarP(&cliOptions.historyTableName, "table", "", "public.migrate_history", "history table name, default to 'public.migrate_history'")

	// TODO: no default value
	cmd.Flags().StringVarP(&cliOptions.dbms, "dbms", "", "postgresql", "dbms vendor: (postgresql, clickhouse)")
}

func RollbackMigrations(cmd *cobra.Command, args []string) {

	//if len(args) != 1 {
	//	cmd.Help()
	//	os.Exit(1)
	//}

	steps := 1 // Default rollback steps
	if len(args) > 0 {
		if num, err := strconv.Atoi(args[0]); err == nil {
			steps = num
		} else {
			fmt.Println("Invalid rollback step. Please provide a number.")
			return
		}
	}
	fmt.Printf("Rolling back %d migrations...\n", steps)
}

func GetLastScripts(cmd *cobra.Command, args []string) {
	fmt.Println("Showing last migration...")
}

func main() {
	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run migrations",
		Run:   RunMigrations,
	}
	addCoreConfigFlagsToCommand(migrateCmd)
	slog.SetDefault(logger.InitLogger(cliOptions.logEnc, cliOptions.logLevel))
	checkRequired()

	rollbackCmd := &cobra.Command{
		Use:   "rollback [steps]",
		Short: "Rollback migrations by a specified number of steps",
		Args:  cobra.MaximumNArgs(1),
		Run:   RollbackMigrations,
	}

	lastCmd := &cobra.Command{
		Use:   "last",
		Short: "Show the last migration",
		Run:   GetLastScripts,
	}

	rootCmd := &cobra.Command{
		Use:   "cli-tool",
		Short: "A CLI tool with migration commands",
	}

	// Add subcommands
	rootCmd.AddCommand(migrateCmd, rollbackCmd, lastCmd)

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
