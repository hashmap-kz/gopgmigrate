package migrate_history

import (
	"context"
)

type MigrateHistoryRepository interface {
	Save(ctx context.Context, inputEntity *MigrateHistoryCreateInput) (*MigrateHistory, error)
	UpdateByID(ctx context.Context, newHash string, pkID int) (*MigrateHistory, error)
	DeleteByID(ctx context.Context, pkID int) error
	FindByID(ctx context.Context, pkID int) (*MigrateHistory, error)
	ExistsByID(ctx context.Context, pkID int) (bool, error)
	FindByName(ctx context.Context, name string) (*MigrateHistory, error)
	FindAll(ctx context.Context) ([]MigrateHistory, error)
	GetAppliedNames(ctx context.Context) (map[string]bool, error)
}
