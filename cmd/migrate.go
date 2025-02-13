package cmd

import (
	"context"
	"database/sql"
	"log/slog"
	"os"

	"gopgmigrate/internal/migrate"

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

	// init repository
	repo, conn := initRepo(ctx)
	defer func(conn *sql.DB) {
		err := conn.Close()
		if err != nil {
			slog.Warn("conn", slog.String("status", err.Error()))
		} else {
			slog.Debug("conn", slog.String("status", "closed:true"))
		}
	}(conn)

	// run all migrations
	err = migrate.RunMigrations(ctx, migrate.RunMigrationCtx{
		MigrateMode:  migrateMode,
		DB:           conn,
		Repo:         repo,
		DirectionDo:  true,
		MigrationDir: cliOptions.dirName,
		DryRun:       dryRun,
	})
	if err != nil {
		slog.Error("migration error", slog.String("err", err.Error()))
		os.Exit(1)
	}

	slog.Info("migrations applied successfully")
}
