package dbms

import (
	"database/sql"
	"fmt"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

// GetDatabaseConnectionClickhouse initializes a Clickhouse connection
func GetDatabaseConnectionClickhouse(dbURL string) (*sql.DB, error) {
	db, err := sql.Open("clickhouse", dbURL)
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
