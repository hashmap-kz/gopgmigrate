package migrate

import "regexp"

var (
	// example: 00003-users.do.sql
	versionedMigrationRegexDo = regexp.MustCompile(`^\d{5}-.*\.do\.sql$`)

	// example: 00003-users.undo.sql
	versionedMigrationRegexUndo = regexp.MustCompile(`^\d{5}-.*\.undo\.sql$`)

	// any filename with '.r.sql' suffix
	repeatableMigrationRegex = regexp.MustCompile(`(?i)\.r\.sql$`)
)

type migrationFile struct {
	vers int64
	path string
	base string
	dir  string
	data []byte
}

type MigrationCtx struct {
	versioned  []migrationFile
	repeatable []migrationFile
}

type migrationParams struct {
	files []migrationFile
}
