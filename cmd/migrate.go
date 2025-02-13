package cmd

import (
	"context"
	"log/slog"
	"os"

	"gopgmigrate/internal/migrate"

	"github.com/spf13/cobra"
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

	// run all migrations
	err = migrate.RunMigrations(ctx, migrate.RunMigrationCtx{
		MigrateMode:      migrateMode,
		DirectionDo:      true,
		MigrationDir:     cliOptions.dirName,
		DryRun:           dryRun,
		ConnStr:          cliOptions.connStr,
		HistoryTableName: cliOptions.historyTableName,
	})
	if err != nil {
		slog.Error("migration error", slog.String("err", err.Error()))
		os.Exit(1)
	}

	slog.Info("migration", slog.String("status", "applied:ok"))
}
