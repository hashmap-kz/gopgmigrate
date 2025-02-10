package migrate

type MigrationFile struct {
	Vers int64
	Path string
	Base string
	data []byte
	hash string
}
