package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"gopgmigrate/internal/migrate"

	"gopgmigrate/pkg/logger"

	"github.com/spf13/cobra"
)

var dryRun bool

var cliOptions struct {
	dirName          string
	connStr          string
	config           string
	logEnc           string
	logLevel         string
	historyTableName string
	dbms             string
}

// Root command
var rootCmd = &cobra.Command{
	Use:   "cli-tool",
	Short: "A CLI tool for database migrations",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize Logger once CLI flags are available
		slog.SetDefault(logger.InitLogger(cliOptions.logEnc, cliOptions.logLevel))

		// Validate required flags
		if cliOptions.dirName == "" {
			cliOptions.dirName = os.Getenv("PGMIGRATE_DIRNAME")
			if cliOptions.dirName == "" {
				return fmt.Errorf("--dirname is required")
			}
		}
		if cliOptions.connStr == "" {
			cliOptions.connStr = os.Getenv("PGMIGRATE_CONNSTR")
			if cliOptions.connStr == "" {
				return fmt.Errorf("--connstr is required")
			}
		}
		if cliOptions.historyTableName == "" {
			cliOptions.historyTableName = os.Getenv("PGMIGRATE_HISTORY_TABLE_NAME")
			if cliOptions.historyTableName == "" {
				return fmt.Errorf("--history-table is required")
			}
			if !migrate.IsSchemaTablePath(cliOptions.historyTableName) {
				return fmt.Errorf("--history-table expected required in format: `schema.table`")
			}
		}
		return nil
	},
}

// Execute the CLI
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Define persistent flags (shared across all commands)
	rootCmd.PersistentFlags().StringVar(&cliOptions.dirName, "dirname", "", "Directory containing migration files (required)")
	rootCmd.PersistentFlags().StringVar(&cliOptions.connStr, "connstr", "", strings.TrimSpace(`
Database connection string (required)
postgresql: postgres://username:password@host:port/dbname
clickhouse: clickhouse://username:password@host:port/dbname`))
	rootCmd.PersistentFlags().StringVar(&cliOptions.config, "config", "", "Path to configuration file (optional)")
	rootCmd.PersistentFlags().StringVar(&cliOptions.logEnc, "log-enc", "text", "Log encoding format (json/text)")
	rootCmd.PersistentFlags().StringVar(&cliOptions.logLevel, "log-level", "debug", "Log level (debug/info/warn/error)")
	rootCmd.PersistentFlags().StringVar(&cliOptions.historyTableName, "history-table", "public.migrate_history", "Migration history table name")
	rootCmd.PersistentFlags().StringVar(&cliOptions.dbms, "dbms", "postgresql", "Database management system (postgresql/clickhouse)")

	// TODO:
	// Mark required flags
	// _ = rootCmd.MarkPersistentFlagRequired("dirname")
	// _ = rootCmd.MarkPersistentFlagRequired("connstr")
}
