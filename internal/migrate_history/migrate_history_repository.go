package migrate_history

import (
	"context"
)

type MigrateHistoryRepository interface {
	Save(ctx context.Context, inputEntity *MigrateHistory) (*MigrateHistory, error)
	UpdateByID(ctx context.Context, inputEntity *MigrateHistory, pkID int) (*MigrateHistory, error)
	DeleteByID(ctx context.Context, pkID int) error
	FindByID(ctx context.Context, pkID int) (*MigrateHistory, error)
	FindAll(ctx context.Context) ([]MigrateHistory, error)
}
