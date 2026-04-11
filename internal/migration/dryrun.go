package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"gopgmigrate/internal/version"
)

func printMigrationsInfo(pendingMigrations []version.MigrationFile) {
	printPendingPlainMode(pendingMigrations)
}

func printPendingPlainMode(migrations []version.MigrationFile) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.TabIndent)
	_, _ = fmt.Fprintln(w, "VERSION\tNAME\tPATH")
	for _, p := range migrations {
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\n", p.Vers, p.Base, filepath.ToSlash(p.Path))
	}
	_ = w.Flush()
}
