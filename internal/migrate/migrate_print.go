package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"gopgmigrate/internal/vers"
)

func printMigrationsInfo(migrateMode string, pendingMigrations []vers.MigrationFile) {
	if migrateMode == ModeMixed {
		printPendingMixedMode(pendingMigrations)
	} else {
		printPendingPlainMode(pendingMigrations)
	}
}

func printPendingPlainMode(migrations []vers.MigrationFile) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.TabIndent)
	_, _ = fmt.Fprintln(w, "VERSION\tNAME\tPATH")
	for _, p := range migrations {
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\n", p.Vers, p.Base, filepath.ToSlash(p.Path))
	}
	_ = w.Flush()
}

func printPendingMixedMode(migrations []vers.MigrationFile) {
	entries, err := ParseFilesMixedMode(migrations)
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
