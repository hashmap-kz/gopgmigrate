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
)

var migrateMode string

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Run:   runMigrations,
}

func init() {
	migrateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate migration execution without applying changes")
	migrateCmd.Flags().StringVar(&migrateMode, "mode", migrate.ModePlain, "Migration mode: plain/group/mixed")
	rootCmd.AddCommand(migrateCmd)
}

func runMigrations(cmd *cobra.Command, args []string) {
	var err error
	ctx := context.Background()

	//////////////////////////////////////////////////////////////////////
	// init repository
	repo, conn, noTxPatterns := initRepo(ctx)
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
	pendingMigrations, err := migrate.GetPendingMigrations(ctx, conn, cliOptions.dirName, noTxPatterns, repo)
	if err != nil {
		slog.Error("collecting pending migrations error", slog.String("err", err.Error()))
		os.Exit(1)
	}

	if dryRun {
		_ = logger.DisableLogging()
		if migrateMode == migrate.ModeMixed {
			printPendingGroups(pendingMigrations)
		} else {
			printPending(pendingMigrations)
		}
		return
	}

	//////////////////////////////////////////////////////////////////////
	// run all migrations
	err = migrate.RunMigrations(ctx, migrateMode, conn, repo, pendingMigrations, true)
	if err != nil {
		slog.Error("migration error", slog.String("err", err.Error()))
		os.Exit(1)
	}

	slog.Info("migrations applied successfully")
}

func printPending(migrations []migrate.MigrationFile) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.TabIndent)
	_, _ = fmt.Fprintln(w, "VERSION\tNAME\tPATH")
	for _, p := range migrations {
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\n", p.Vers, p.Base, filepath.ToSlash(p.Path))
	}
	_ = w.Flush()
}

func printPendingGroups(migrations []migrate.MigrationFile) {
	entries, err := migrate.ParseFilesMixedMode(migrations)
	if err != nil {
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.TabIndent)
	_, _ = fmt.Fprintln(w, "VERSION\tNAME\tPATH")
	for batchIdx, batch := range entries {
		_, _ = fmt.Fprintf(w, "GID:%d\tTX:%v\t\n", batchIdx+1, batch.UseTX)
		for _, p := range batch.Files {
			_, _ = fmt.Fprintf(w, "%d\t%s\t%s\n", p.Vers, p.Base, filepath.ToSlash(p.Path))
		}
	}

	_ = w.Flush()
}
