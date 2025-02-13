package impl

import (
	"context"
	"database/sql"
	"fmt"

	"gopgmigrate/internal/dbms"
	"gopgmigrate/internal/history"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Migration Lock Key (must be unique per application)
const migrationLockKey = 123456

type migrateHistoryPostgresRepository struct {
	tableName string
}

var _ history.MigrateHistoryRepository = &migrateHistoryPostgresRepository{}

func NewMigrateHistoryPostgresRepository(_ context.Context, tableName string) history.MigrateHistoryRepository {
	return &migrateHistoryPostgresRepository{
		tableName: tableName,
	}
}

func (r *migrateHistoryPostgresRepository) CreateHistoryTable(ctx context.Context, tx dbms.Transaction) error {
	tag := "migrateHistoryPostgresRepository.CreateHistoryTable"

	query := fmt.Sprintf(`
    create table if not exists %s
    (
      id            int generated always as identity primary key,
      mh_version    bigint unique not null,
      mh_name       text unique   not null,
      mh_hash       text          not null,
      mh_applied_by name          not null default session_user,
      mh_applied_at timestamptz   not null default transaction_timestamp(),
      mh_txid     text            not null default pg_current_xact_id()::text,
      constraint check_version_match_name check (left(mh_name, 5)::integer = mh_version),
      constraint check_version_unsigned   check (mh_version >= 0 ),
      constraint check_filename           check (mh_name ~ '^(\d{5})-([[:alnum:]_-]+)(?:\.ntx)?\.(do|r)\.sql$')
    );
  `, r.tableName)

	_, err := tx.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	return err
}

func (r *migrateHistoryPostgresRepository) SaveVersioned(ctx context.Context, tx dbms.Transaction, inputEntity *history.MigrateHistoryCreateInput) error {
	tag := "migrateHistoryPostgresRepository.SaveVersioned"
	query := fmt.Sprintf(`		
		insert into %s (
			mh_version,
			mh_name,
			mh_hash
		)
		values ($1, $2, $3)
		returning
			id,
			mh_version,
			mh_name,
			mh_hash,
			mh_applied_by,
			mh_applied_at,
			mh_txid
		`, r.tableName)
	_, err := tx.ExecContext(ctx, query, inputEntity.MhVersion, inputEntity.MhName, inputEntity.MhHash)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	return nil
}

func (r *migrateHistoryPostgresRepository) SaveRepeatable(ctx context.Context, tx dbms.Transaction, inputEntity *history.MigrateHistoryCreateInput) error {
	tag := "migrateHistoryPostgresRepository.SaveRepeatable"
	query := fmt.Sprintf(`    
    with updated as (
      update %s
        set 
          mh_hash       = $3,
          mh_applied_by = session_user,
          mh_applied_at = transaction_timestamp(),
          mh_txid       = pg_current_xact_id()::text
        where mh_name   = $2
        returning id
    )
    insert
    into %s (mh_version, mh_name, mh_hash, mh_applied_by, mh_applied_at)
    select $1, $2, $3, session_user, transaction_timestamp()
    where not exists (select 1 from updated)
    `, r.tableName, r.tableName)
	_, err := tx.ExecContext(ctx, query, inputEntity.MhVersion, inputEntity.MhName, inputEntity.MhHash)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	return nil
}

func (r *migrateHistoryPostgresRepository) ListAll(ctx context.Context, tx dbms.Transaction) ([]history.MigrateHistory, error) {
	tag := "migrateHistoryPostgresRepository.ListAll"

	query := fmt.Sprintf(`		
		select
			id,
			mh_version,
			mh_name,
			mh_hash,
			mh_applied_by,
			mh_applied_at,
			mh_txid
		from %s
		order by mh_version
	`, r.tableName)

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tag, err)
	}
	defer rows.Close()

	var scannedEntities []history.MigrateHistory
	for rows.Next() {
		scannedEntity, err := scanFullRow(rows)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", tag, err)
		}
		scannedEntities = append(scannedEntities, *scannedEntity)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return scannedEntities, nil
}

func (r *migrateHistoryPostgresRepository) DeleteVersion(ctx context.Context, tx dbms.Transaction, scriptName string) error {
	tag := "migrateHistoryPostgresRepository.DeleteVersion"
	query := fmt.Sprintf(`
		delete from only %s
		where mh_name = $1
	`, r.tableName)
	_, err := tx.ExecContext(ctx, query, scriptName)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	return nil
}

// locks

// AcquireMigrationLock ensures only one migration process runs at a time
func (r *migrateHistoryPostgresRepository) AcquireMigrationLock(ctx context.Context, db dbms.Transaction) (bool, error) {
	var acquired bool
	err := db.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", migrationLockKey).Scan(&acquired)
	return acquired, err
}

// ReleaseMigrationLock releases the advisory lock
func (r *migrateHistoryPostgresRepository) ReleaseMigrationLock(ctx context.Context, db dbms.Transaction) error {
	_, err := db.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", migrationLockKey)
	return err
}

// scan utils

// scanFullRow is expected to scan all columns from a table.
// For simplicity, most methods scan the entire row of the table into the result entity.
// You should adapt methods as needed (e.g., if business logic requires returning only an ID after an UPDATE).
func scanFullRow(row *sql.Rows) (*history.MigrateHistory, error) {
	var scannedEntity history.MigrateHistory
	err := row.Scan(
		&scannedEntity.ID,
		&scannedEntity.MhVersion,
		&scannedEntity.MhName,
		&scannedEntity.MhHash,
		&scannedEntity.MhAppliedBy,
		&scannedEntity.MhAppliedAt,
		&scannedEntity.MhTxid,
	)
	if err != nil {
		return nil, err
	}
	return &scannedEntity, nil
}
