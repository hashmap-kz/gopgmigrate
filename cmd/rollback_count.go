package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"gopgmigrate/internal/migration"

	"github.com/spf13/cobra"
)

var rollbackConfirmTwice bool

var rollbackCmd = &cobra.Command{
	Use:   "rollback-count [steps]",
	Short: "Rollback database migrations",
	Args:  cobra.ExactArgs(1),
	Run:   runRollback,
}

func init() {
	rollbackCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate rollback execution without applying changes")
	rollbackCmd.Flags().BoolVar(&rollbackConfirmTwice, "yes-i-really-mean-it", false, "Confirm twice before doing the real rollback")
	rootCmd.AddCommand(rollbackCmd)
}

func runRollback(cmd *cobra.Command, args []string) {
	var err error
	ctx := context.Background()

	steps := 1
	if num, err := strconv.Atoi(args[0]); err == nil {
		steps = num
	} else {
		fmt.Println("Invalid rollback step. Please provide a number.")
		return
	}

	if steps <= 0 {
		fmt.Println("Invalid rollback step. Please provide a number.")
		return
	}

	if !rollbackConfirmTwice && !dryRun {
		fmt.Println("You should confirm that you're really want to rollback with adding '--yes-i-really-mean-it'. You may also check --dry-run, before applying")
		return
	}

	// run all migrations
	err = migration.RunMigrations(ctx, migration.RunMigrationCtx{
		MigrationDir:     cliOptions.dirName,
		DryRun:           dryRun,
		ConnStr:          cliOptions.connStr,
		HistoryTableName: cliOptions.historyTableName,

		DirectionDo: false,
		UndoCount:   steps,
	})
	if err != nil {
		slog.Error("migration", slog.String("err", err.Error()))
		os.Exit(1)
	}

	slog.Info("migration", slog.String("status", "applied:ok"))
}
