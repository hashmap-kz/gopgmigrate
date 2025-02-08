package migrate

import "regexp"

const (
	schemaDirName           = "schema"
	repeatableDirName       = "repeatable"
	dataDirName             = "data"
	defaultHistoryTableName = "public.migrate_history"
)

var (
	// example: 00003-users.do.sql
	versionedMigrationRegexDo = regexp.MustCompile(`^\d{5}-.*\.do\.sql$`)

	// example: 00003-users.undo.sql
	versionedMigrationRegexUndo = regexp.MustCompile(`^\d{5}-.*\.undo\.sql$`)
)

type migrationFile struct {
	path string
	base string
	dir  string
	data []byte
}

type MigrationCtx struct {
	schema     []migrationFile
	repeatable []migrationFile
	data       []migrationFile
}

type migrationParams struct {
	mode  string
	files []migrationFile
}
