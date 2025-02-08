package migrate_history

type MigrateHistorySearchNameMode struct {
	MhName string
	MhMode string
}

type MigrateHistoryCreateInput struct {
	MhVersion int64
	MhMode    string
	MhName    string
	MhHash    string
}
