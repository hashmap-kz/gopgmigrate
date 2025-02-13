package dbms

import (
	"database/sql"
	"fmt"
	"regexp"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// GetDatabaseConnectionPostgres initializes a PostgreSQL connection
func GetDatabaseConnectionPostgres(dbURL string) (*sql.DB, error) {
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

func GetNoTxPatternsPostgres() map[string]*regexp.Regexp {
	return map[string]*regexp.Regexp{
		"CopyFromStdin":                        regexp.MustCompile(`(?i)COPY( .*)? FROM STDIN`),
		"CreateDatabaseTablespaceSubscription": regexp.MustCompile(`(?i)(CREATE|DROP) (DATABASE|TABLESPACE|SUBSCRIPTION)`),
		"AlterSystem":                          regexp.MustCompile(`(?i)ALTER SYSTEM`),
		"CreateIndexConcurrently":              regexp.MustCompile(`(?i)(CREATE|DROP)( UNIQUE)? INDEX CONCURRENTLY`),
		"Reindex":                              regexp.MustCompile(`(?i)REINDEX( VERBOSE)? (SCHEMA|DATABASE|SYSTEM)`),
		"Vacuum":                               regexp.MustCompile(`(?i)VACUUM`),
		"DiscardAll":                           regexp.MustCompile(`(?i)DISCARD ALL`),
		"AlterTypeAddValue":                    regexp.MustCompile(`(?i)ALTER TYPE( .*)? ADD VALUE`),
	}
}
