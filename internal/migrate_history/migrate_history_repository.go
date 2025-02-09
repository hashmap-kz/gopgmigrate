package migrate_history

import (
	"context"
)

type MigrateHistoryRepository interface {
	CreateHistoryTable(ctx context.Context) error
	SaveVersioned(ctx context.Context, inputEntity *MigrateHistoryVersionedCreateInput) error
	SaveRepeatable(ctx context.Context, inputEntity *MigrateHistoryRepeatableCreateInput) error
	ListAll(ctx context.Context) ([]MigrateHistory, error)
}
