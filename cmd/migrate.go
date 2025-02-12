package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"text/tabwriter"

	"gopgmigrate/internal/migrate"
	"gopgmigrate/pkg/logger"

	"github.com/spf13/cobra"
)

const (
	dbmsVendorPostgresql = "postgresql"
	dbmsVendorClickhouse = "clickhouse"

	// MigrateModeGroup applies all pending migrations as a single "group".
	// This means that all migrations must either be executed within a single transaction (if they are transactional)
	// or all must be non-transactional.
	migrateModeGroup string = "group"

	// MigrateModeMixed applies all pending migrations in separate transactional and non-transactional groups.
	// Migrations are divided into list of groups: each group contains list of files transactional or non-transactional, and each group is executed separately.
	migrateModeMixed string = "mixed"

	// MigrateModePlain executes migrations one by one, without grouping.
	// Each migration script is applied individually in sequence.
	migrateModePlain string = "plain"
)

var migrateMode string

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Run:   runMigrations,
}

func init() {
	migrateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate migration execution without applying changes")
	migrateCmd.Flags().StringVar(&migrateMode, "mode", migrateModePlain, "Migration mode: plain/group/mixed")
	rootCmd.AddCommand(migrateCmd)
}

func runMigrations(cmd *cobra.Command, args []string) {
	var err error
	ctx := context.Background()

	//////////////////////////////////////////////////////////////////////
	// get migration scripts
	files, err := migrate.GetFiles(cliOptions.dirName)
	if err != nil {
		slog.Error("collecting files error", slog.String("err", err.Error()))
		os.Exit(1)
	}

	//////////////////////////////////////////////////////////////////////
	// init repository
	repo, conn := getRepoAndConn(ctx)
	defer func(conn *sql.DB) {
		err := conn.Close()
		if err != nil {
			slog.Warn("conn", slog.String("status", err.Error()))
		} else {
			slog.Debug("conn", slog.String("status", "closed:true"))
		}
	}(conn)

	//////////////////////////////////////////////////////////////////////
	// get pending migrations
	pendingMigrations, err := migrate.GetPendingMigrations(ctx, conn, files, repo)
	if err != nil {
		slog.Error("collecting pending migrations error", slog.String("err", err.Error()))
		os.Exit(1)
	}

	if dryRun {
		_ = logger.DisableLogging()
		if migrateMode == migrateModeMixed {
			printPendingGroups(pendingMigrations)
		} else {
			printPending(pendingMigrations)
		}
		return
	}

	//////////////////////////////////////////////////////////////////////
	// acquire advisory lock
	acquired, err := repo.AcquireMigrationLock(ctx, conn)
	if err != nil {
		slog.Error("unable to acquire lock", slog.String("err", err.Error()))
		os.Exit(1)
	}
	if !acquired {
		slog.Error("another migration process is running. exiting.")
		os.Exit(1)
	}
	slog.Debug("lock", slog.String("status", "acquired:true"))
	defer func(ctx context.Context, conn *sql.DB) {
		err = repo.ReleaseMigrationLock(ctx, conn)
		if err != nil {
			slog.Warn("lock", slog.String("status", err.Error()))
		} else {
			slog.Debug("lock", slog.String("status", "released:true"))
		}
	}(ctx, conn)

	//////////////////////////////////////////////////////////////////////
	// run all migrations
	if migrateMode == migrateModeMixed {
		batchEntries, err := migrate.ParseFilesMixedMode(pendingMigrations)
		if err != nil {
			slog.Error("migration error", slog.String("err", err.Error()))
			os.Exit(1)
		}
		err = migrate.RunMigrationsMixedMode(ctx, conn, repo, batchEntries, true)
		if err != nil {
			slog.Error("migration error", slog.String("err", err.Error()))
			os.Exit(1)
		}
	} else if migrateMode == migrateModePlain {
		err = migrate.RunMigrationsPlainMode(ctx, conn, repo, pendingMigrations, true)
		if err != nil {
			slog.Error("migration error", slog.String("err", err.Error()))
			os.Exit(1)
		}
	} else if migrateMode == migrateModeGroup {
		groupEntry, err := migrate.ParseFilesGroupMode(pendingMigrations)
		if err != nil {
			slog.Error("migration error", slog.String("err", err.Error()))
			os.Exit(1)
		}
		err = migrate.RunMigrationsGroupMode(ctx, conn, repo, groupEntry, true)
		if err != nil {
			slog.Error("migration error", slog.String("err", err.Error()))
			os.Exit(1)
		}
	} else {
		slog.Error("migration error", slog.String("unknown-mode", migrateMode))
		os.Exit(1)
	}

	slog.Info("migrations applied successfully")
}

func printPending(migrations []migrate.MigrationFile) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.TabIndent)
	printEntries(migrations, w)
	_ = w.Flush()
}

func printEntries(migrations []migrate.MigrationFile, w *tabwriter.Writer) {
	_, _ = fmt.Fprintln(w, "VERSION\tNAME\tPATH")
	for _, p := range migrations {
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\n", p.Vers, p.Base, filepath.ToSlash(p.Path))
	}
}

func printPendingGroups(migrations []migrate.MigrationFile) {
	entries, err := migrate.ParseFilesMixedMode(migrations)
	if err != nil {
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.TabIndent)
	for batchIdx, batch := range entries {
		_, _ = fmt.Fprintf(w, "\n-------------- GroupID: %d UseTX: %v --------------\n", batchIdx, batch.UseTX)
		printEntries(batch.Files, w)
	}

	_ = w.Flush()
}
