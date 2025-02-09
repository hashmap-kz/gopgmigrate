package migrate_history

import (
	"context"
)

type MigrateHistoryRepository interface {
	SaveVersioned(ctx context.Context, inputEntity *MigrateHistoryVersionedCreateInput) (*MigrateHistory, error)
	SaveRepeatable(ctx context.Context, inputEntity *MigrateHistoryRepeatableCreateInput) (*MigrateHistory, error)
	SaveOrUpdateRepeatable(ctx context.Context, inputEntity *MigrateHistoryRepeatableCreateInput) (*MigrateHistory, error)
	UpdateByName(ctx context.Context, newHash string, name string) (*MigrateHistory, error)
	ExistsByName(ctx context.Context, name string) (bool, error)
	FindByName(ctx context.Context, name string) (*MigrateHistory, error)
	FindAll(ctx context.Context) ([]MigrateHistory, error)
	GetAppliedNames(ctx context.Context) (map[string]bool, error)
}
