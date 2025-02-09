package migrate_history

import (
	"context"
)

type MigrateHistoryRepository interface {
	Save(ctx context.Context, inputEntity *MigrateHistoryVersionedCreateInput) (*MigrateHistory, error)
	SaveOrUpdate(ctx context.Context, inputEntity *MigrateHistoryRepeatableCreateInput) (*MigrateHistory, error)
	UpdateByName(ctx context.Context, newHash string, name string) (*MigrateHistory, error)
	DeleteByID(ctx context.Context, pkID int) error
	FindByID(ctx context.Context, pkID int) (*MigrateHistory, error)
	ExistsByName(ctx context.Context, name string) (bool, error)
	FindByName(ctx context.Context, name string) (*MigrateHistory, error)
	FindAll(ctx context.Context) ([]MigrateHistory, error)
	GetAppliedNames(ctx context.Context) (map[string]bool, error)
}
