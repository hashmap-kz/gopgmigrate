package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var lastCmd = &cobra.Command{
	Use:   "last",
	Short: "Show the last applied migration",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Last migration applied in '%s' using connection: %s\n",
			cliOptions.dirName, cliOptions.connStr)
	},
}

func init() {
	rootCmd.AddCommand(lastCmd)
}
