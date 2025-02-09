package migrate

type migrationFile struct {
	path string
	base string
	data []byte
}
