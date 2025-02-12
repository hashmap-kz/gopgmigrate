package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback [steps]",
	Short: "Rollback database migrations",
	Args:  cobra.MaximumNArgs(1),
	Run:   runRollback,
}

func init() {
	rollbackCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate rollback execution without applying changes")
	rootCmd.AddCommand(rollbackCmd)
}

func runRollback(cmd *cobra.Command, args []string) {
	steps := 1
	if len(args) > 0 {
		if num, err := strconv.Atoi(args[0]); err == nil {
			steps = num
		} else {
			fmt.Println("Invalid rollback step. Please provide a number.")
			return
		}
	}
	if steps <= 0 {
		fmt.Println("Invalid rollback step. Please provide a number.")
		return
	}

	fmt.Printf("Rolling back %d migrations in '%s' using connection: %s\n",
		steps, cliOptions.dirName, cliOptions.connStr)
}
