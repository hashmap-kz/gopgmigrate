package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"gopgmigrate/internal/migration"
	"gopgmigrate/internal/naming"
	"gopgmigrate/pkg/logger"
)

type cliOptions struct {
	dirName          string
	connStr          string
	logEnc           string
	logLevel         string
	historyTableName string
}

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		printUsage()
		return 1
	}

	switch os.Args[1] {
	case "migrate":
		return runMigrate(os.Args[2:])
	case "last":
		return runLast(os.Args[2:])
	case "rollback-count":
		return runRollbackCount(os.Args[2:])
	case "-h", "--help", "help":
		printUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		return 1
	}
}

func runMigrate(args []string) int {
	var opts cliOptions
	var dryRun bool

	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	registerCommonFlags(fs, &opts)
	fs.BoolVar(&dryRun, "dry-run", false, "Simulate migration execution without applying changes")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if err := prepareOptions(&opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	ctx := context.Background()
	err := migration.RunMigrations(ctx, migration.RunMigrationCtx{
		DirectionDo:      true,
		MigrationDir:     opts.dirName,
		DryRun:           dryRun,
		ConnStr:          opts.connStr,
		HistoryTableName: opts.historyTableName,
	})
	if err != nil {
		slog.Error("migration", slog.String("err", err.Error()))
		return 1
	}

	slog.Info("migration", slog.String("status", "applied:ok"))
	return 0
}

func runLast(args []string) int {
	var opts cliOptions

	fs := flag.NewFlagSet("last", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	registerCommonFlags(fs, &opts)

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if err := prepareOptions(&opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Printf("Last migration applied in '%s' using connection: %s\n", opts.dirName, opts.connStr)
	return 0
}

func runRollbackCount(args []string) int {
	var opts cliOptions
	var dryRun bool

	fs := flag.NewFlagSet("rollback-count", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	registerCommonFlags(fs, &opts)
	fs.BoolVar(&dryRun, "dry-run", false, "Simulate rollback execution without applying changes")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if err := prepareOptions(&opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "rollback-count requires exactly 1 argument: [steps]")
		fs.Usage()
		return 1
	}

	steps, err := strconv.Atoi(fs.Arg(0))
	if err != nil || steps <= 0 {
		fmt.Fprintln(os.Stderr, "invalid rollback step. Please provide a positive number.")
		return 1
	}

	ctx := context.Background()
	err = migration.RunMigrations(ctx, migration.RunMigrationCtx{
		MigrationDir:     opts.dirName,
		DryRun:           dryRun,
		ConnStr:          opts.connStr,
		HistoryTableName: opts.historyTableName,
		DirectionDo:      false,
		UndoCount:        steps,
	})
	if err != nil {
		slog.Error("migration", slog.String("err", err.Error()))
		return 1
	}

	slog.Info("migration", slog.String("status", "applied:ok"))
	return 0
}

func registerCommonFlags(fs *flag.FlagSet, opts *cliOptions) {
	fs.StringVar(&opts.dirName, "dirname", "", "Directory containing migration files (required)")
	fs.StringVar(&opts.connStr, "connstr", "", "postgres://username:password@host:port/dbname?key=val")
	fs.StringVar(&opts.logEnc, "log-enc", "text", "Log encoding format (json/text)")
	fs.StringVar(&opts.logLevel, "log-level", "debug", "Log level (debug/info/warn/error)")
	fs.StringVar(&opts.historyTableName, "history-table", "public.migrate_history", "Migration history table name")
}

func prepareOptions(opts *cliOptions) error {
	slog.SetDefault(logger.InitLogger(opts.logEnc, opts.logLevel))

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
		return fmt.Errorf("--history-table expected required in format: `schema.table`")
	}

	return nil
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  gopgmigrate <command> [flags]

Commands:
  migrate         Run database migrations
  last            Show the last applied migration
  rollback-count  Rollback database migrations

Examples:
  gopgmigrate migrate --dirname ./migrations --connstr postgres://user:pass@localhost:5432/db --history-table public.migrate_history
  gopgmigrate last --dirname ./migrations --connstr postgres://user:pass@localhost:5432/db --history-table public.migrate_history
  gopgmigrate rollback-count 2 --dirname ./migrations --connstr postgres://user:pass@localhost:5432/db --history-table public.migrate_history --yes-i-really-mean-it
`)
}
