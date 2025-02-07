package migrate

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// GetDatabaseConnection initializes a PostgreSQL connection
func GetDatabaseConnection(ctx context.Context, dbURL string) (*pgx.Conn, error) {
	return pgx.Connect(ctx, dbURL)
}
