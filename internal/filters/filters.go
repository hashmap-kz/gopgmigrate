package filters

import (
	"fmt"
	"path/filepath"
	"regexp"

	"gopgmigrate/internal/naming"
)

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

func getNoTxPatterns() map[string]*regexp.Regexp {
	return map[string]*regexp.Regexp{
		"CopyFromStdin":                        regexp.MustCompile(`(?i)COPY( .*)? FROM STDIN`),
		"CreateDatabaseTablespaceSubscription": regexp.MustCompile(`(?i)(CREATE|DROP) (DATABASE|TABLESPACE|SUBSCRIPTION)`),
		"AlterSystem":                          regexp.MustCompile(`(?i)ALTER SYSTEM`),
		"CreateIndexConcurrently":              regexp.MustCompile(`(?i)(CREATE|DROP)( UNIQUE)? INDEX CONCURRENTLY`),
		"Reindex":                              regexp.MustCompile(`(?i)REINDEX( VERBOSE)? (SCHEMA|DATABASE|SYSTEM)`),
		"Vacuum":                               regexp.MustCompile(`(?i)VACUUM`),
		"DiscardAll":                           regexp.MustCompile(`(?i)DISCARD ALL`),
		"AlterTypeAddValue":                    regexp.MustCompile(`(?i)ALTER TYPE( .*)? ADD VALUE`),
	}
}

func checkFilesAreUniqueByVersion(versioned []naming.MigrationFile) error {
	seenVersions := map[int64]bool{}
	for _, f := range versioned {
		if _, ok := seenVersions[f.Vers]; ok {
			return fmt.Errorf("%s is used a version that already in use",
				filepath.ToSlash(f.Path),
			)
		}
		seenVersions[f.Vers] = true
	}
	return nil
}
