package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"gopgmigrate/internal/logger"
	"gopgmigrate/internal/migration"
	"gopgmigrate/internal/naming"

	"github.com/spf13/cobra"
)

type cliOptions struct {
	dirName          string
	connStr          string
	logLevel         string
	historyTableName string
}

func main() {
	os.Exit(run())
}

func run() int {
	if err := newRootCmd().Execute(); err != nil {
		return 1
	}
	return 0
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "gopgmigrate",
		Short:         "PostgreSQL migration tool",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newMigrateCmd(),
		newLastCmd(),
		newRollbackCountCmd(),
	)

	return root
}

func newMigrateCmd() *cobra.Command {
	var opts cliOptions
	var dryRun bool

	//nolint:gosec
	cmd := &cobra.Command{
		Use:     "migrate",
		Short:   "Run database migrations",
		Example: "  gopgmigrate migrate --dirname ./migrations --connstr postgres://user:pass@localhost:5432/db --history-table public.migrate_history",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := prepareOptions(&opts); err != nil {
				return err
			}

			ctx := context.Background()
			return migration.RunMigrationsUp(ctx, &migration.ApplyOpts{
				MigrationDir:     opts.dirName,
				DryRun:           dryRun,
				ConnStr:          opts.connStr,
				HistoryTableName: opts.historyTableName,
			})
		},
		PostRunE: func(_ *cobra.Command, _ []string) error {
			slog.Info("migration", slog.String("status", "applied:ok"))
			return nil
		},
	}

	registerCommonFlags(cmd, &opts)
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate migration execution without applying changes")

	return cmd
}

func newLastCmd() *cobra.Command {
	var opts cliOptions

	//nolint:gosec
	cmd := &cobra.Command{
		Use:     "last",
		Short:   "Show the last applied migration",
		Example: "  gopgmigrate last --dirname ./migrations --connstr postgres://user:pass@localhost:5432/db --history-table public.migrate_history",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := prepareOptions(&opts); err != nil {
				return err
			}

			fmt.Printf("Last migration applied in '%s' using connection: %s\n", opts.dirName, opts.connStr)
			return nil
		},
	}

	registerCommonFlags(cmd, &opts)

	return cmd
}

func newRollbackCountCmd() *cobra.Command {
	var opts cliOptions
	var dryRun bool

	//nolint:gosec
	cmd := &cobra.Command{
		Use:     "rollback-count [steps]",
		Short:   "Rollback database migrations by a given number of steps",
		Example: "  gopgmigrate rollback-count 2 --dirname ./migrations --connstr postgres://user:pass@localhost:5432/db --history-table public.migrate_history",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("requires exactly 1 argument: [steps]")
			}
			steps, err := strconv.Atoi(args[0])
			if err != nil || steps <= 0 {
				return fmt.Errorf("invalid rollback step: must be a positive number")
			}
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			if err := prepareOptions(&opts); err != nil {
				return err
			}

			//nolint:errcheck
			steps, _ := strconv.Atoi(args[0])

			ctx := context.Background()
			return migration.RunMigrationsDown(ctx, &migration.RollbackOpts{
				MigrationDir:     opts.dirName,
				DryRun:           dryRun,
				ConnStr:          opts.connStr,
				HistoryTableName: opts.historyTableName,
				UndoCount:        steps,
			})
		},
		PostRunE: func(_ *cobra.Command, _ []string) error {
			slog.Info("migration", slog.String("status", "applied:ok"))
			return nil
		},
	}

	registerCommonFlags(cmd, &opts)
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate rollback execution without applying changes")

	return cmd
}

func registerCommonFlags(cmd *cobra.Command, opts *cliOptions) {
	cmd.Flags().StringVar(&opts.dirName, "dirname", "", "Directory containing migration files (required)")
	cmd.Flags().StringVar(&opts.connStr, "connstr", "", "postgres://username:password@host:port/dbname?key=val")
	cmd.Flags().StringVar(&opts.logLevel, "log-level", "info", "Log level (debug/info/warn/error)")
	cmd.Flags().StringVar(&opts.historyTableName, "history-table", "public.migrate_history", "Migration history table name")
}

func prepareOptions(opts *cliOptions) error {
	logger.Init(&logger.Opts{
		Level:     opts.logLevel,
		Format:    "text",
		AddSource: false,
	})

	if opts.dirName == "" {
		opts.dirName = os.Getenv("PGMIGRATE_DIRNAME")
		if opts.dirName == "" {
			return fmt.Errorf("--dirname is required")
		}
	}

	if opts.connStr == "" {
		opts.connStr = os.Getenv("PGMIGRATE_CONNSTR")
		if opts.connStr == "" {
			return fmt.Errorf("--connstr is required")
		}
	}

	if opts.historyTableName == "" {
		opts.historyTableName = os.Getenv("PGMIGRATE_HISTORY_TABLE_NAME")
		if opts.historyTableName == "" {
			return fmt.Errorf("--history-table is required")
		}
	}

	if !naming.IsSchemaTablePath(opts.historyTableName) {
		return fmt.Errorf("--history-table must be in format: `schema.table`")
	}

	return nil
}
