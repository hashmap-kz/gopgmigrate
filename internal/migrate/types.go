package migrate

type migrationFile struct {
	vers int64
	path string
	base string
	data []byte
	hash string
}
