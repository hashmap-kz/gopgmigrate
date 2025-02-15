package cmd

import (
	"context"
	"log/slog"
	"os"

	"gopgmigrate/internal/mode"

	"gopgmigrate/internal/migration"

	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Run:   runMigrations,
}

func init() {
	migrateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate migration execution without applying changes")
	migrateCmd.Flags().StringVar(&migrateMode, "mode", mode.ModePlain, "Migration mode: plain/group/mixed")
	rootCmd.AddCommand(migrateCmd)
}

func runMigrations(cmd *cobra.Command, args []string) {
	var err error
	ctx := context.Background()

	// run all migrations
	err = migration.RunMigrations(ctx, migration.RunMigrationCtx{
		MigrateMode:      migrateMode,
		DirectionDo:      true,
		MigrationDir:     cliOptions.dirName,
		DryRun:           dryRun,
		ConnStr:          cliOptions.connStr,
		HistoryTableName: cliOptions.historyTableName,
	})
	if err != nil {
		slog.Error("migration", slog.String("err", err.Error()))
		os.Exit(1)
	}

	slog.Info("migration", slog.String("status", "applied:ok"))
}
