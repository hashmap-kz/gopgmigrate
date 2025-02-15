package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"gopgmigrate/internal/mode"

	"gopgmigrate/internal/version"
)

func printMigrationsInfo(migrateMode string, pendingMigrations []version.MigrationFile) {
	if migrateMode == mode.ModeMixed {
		printPendingMixedMode(pendingMigrations)
	} else {
		printPendingPlainMode(pendingMigrations)
	}
}

func printPendingPlainMode(migrations []version.MigrationFile) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.TabIndent)
	_, _ = fmt.Fprintln(w, "VERSION\tNAME\tPATH")
	for _, p := range migrations {
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\n", p.Vers, p.Base, filepath.ToSlash(p.Path))
	}
	_ = w.Flush()
}

func printPendingMixedMode(migrations []version.MigrationFile) {
	entries, err := mode.ParseFilesMixedMode(migrations)
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
