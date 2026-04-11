package filters

import "gopgmigrate/internal/naming"

func filterMigrationFiles(
	files []naming.MigrationFile,
	keep func(naming.MigrationFile) bool,
) []naming.MigrationFile {
	out := make([]naming.MigrationFile, 0, len(files))
	for _, f := range files {
		if keep(f) {
			out = append(out, f)
		}
	}
	return out
}
