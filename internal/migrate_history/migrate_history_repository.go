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
	FindByNameMode(ctx context.Context, searchDTO MigrateHistorySearchNameMode) (*MigrateHistory, error)
	FindAll(ctx context.Context) ([]MigrateHistory, error)
	FindAllByMode(ctx context.Context, mode string) ([]MigrateHistory, error)
}
