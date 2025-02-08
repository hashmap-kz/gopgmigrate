package migrate_history

type MigrateHistoryCreateInput struct {
	MhVersion int64
	MhName    string
	MhHash    string
}
