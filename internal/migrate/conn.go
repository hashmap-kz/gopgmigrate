package migrate

import (
	"database/sql"
	"fmt"

	// TODO: drivers package (clickhouse, postgres)
	_ "github.com/jackc/pgx/v5/stdlib" // Import for database/sql compatibility
)

// GetDatabaseConnection initializes a PostgreSQL connection
func GetDatabaseConnection(dbURL string) (*sql.DB, error) {
	// Open connection using pgx's stdlib adapter
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Verify the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}
