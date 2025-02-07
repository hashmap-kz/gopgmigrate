package migrate

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// EnsureSchemaMigrationTables checks that migration tracking tables exist
func EnsureSchemaMigrationTables(conn *pgx.Conn) error {
	query := fmt.Sprintf(`
		create table if not exists %s
		(
			id         serial primary key,
			version    int,
			mode       varchar(16) not null,
			name       text        not null,
			hash       text        not null,
			applied_at timestamp default now(),
			constraint ckh_mode check ( mode in ('schema', 'data', 'repeatable') )
		);
		create unique index if not exists ix_migrate_history_unq on %s (mode, name);
	`, defaultHistoryTableName, defaultHistoryTableName)
	_, err := conn.Exec(context.Background(), query)
	return err
}
