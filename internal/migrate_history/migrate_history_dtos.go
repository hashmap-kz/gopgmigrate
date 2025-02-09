package migrate_history

type MigrateHistoryVersionedCreateInput struct {
	MhVersion int64
	MhName    string
	MhHash    string
}

type MigrateHistoryRepeatableCreateInput struct {
	MhName string
	MhHash string
}
