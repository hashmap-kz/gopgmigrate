package migrate

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
