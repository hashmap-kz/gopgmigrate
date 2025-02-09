package migrate

type migrationFile struct {
	path string
	base string
	data []byte
	notx bool
}

type MigrationCtx struct {
	versioned  []migrationFile
	repeatable []migrationFile
}
