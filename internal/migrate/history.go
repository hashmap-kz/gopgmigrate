package migrate

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// EnsureSchemaMigrationTables checks that migration tracking tables exist
func EnsureSchemaMigrationTables(conn *pgx.Conn) error {
	query := `
		create table if not exists public.migrate_history
		(
			id            int 		  generated always as identity primary key,
			mh_version    bigint 	  unique,
			mh_name       text        unique not null,
			mh_hash       text        not null,
			mh_applied_by name        not null default session_user,
			mh_applied_at timestamptz not null default transaction_timestamp()
		);
		create unique index if not exists ix_migrate_history_ver_name_unq on public.migrate_history(mh_version, mh_name);

		comment on table  public.migrate_history is 'Tracks executed migrations, ensuring version control and repeatable migrations.';
		comment on column public.migrate_history.id is 'Auto-incrementing primary key.';
		comment on column public.migrate_history.mh_version is 'Version number of the migration (bigint). Used for versioned migrations.';
		comment on column public.migrate_history.mh_name is 'Name of the migration file applied.';
		comment on column public.migrate_history.mh_hash is 'SHA256 hash of the migration script to detect changes in repeatable migrations.';
		comment on column public.migrate_history.mh_applied_by is 'User who executed the migration.';
		comment on column public.migrate_history.mh_applied_at is 'Timestamp when the migration was applied.';
	`
	_, err := conn.Exec(context.Background(), query)
	return err
}
