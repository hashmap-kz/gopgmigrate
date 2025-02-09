package migrate

type migrationFile struct {
	path string
	base string
	data []byte
}

type MigrationCtx struct {
	versioned  []migrationFile
	repeatable []migrationFile
}
