package impl

import (
	"context"
	"database/sql"
	"fmt"

	"gopgmigrate/internal/history"
)

type migrateHistoryPostgresRepository struct {
	tableName string
}

var _ history.MigrateHistoryRepository = &migrateHistoryPostgresRepository{}

func NewMigrateHistoryPostgresRepository(_ context.Context, tableName string) history.MigrateHistoryRepository {
	return &migrateHistoryPostgresRepository{
		tableName: tableName,
	}
}

func (r *migrateHistoryPostgresRepository) CreateHistoryTable(ctx context.Context, tx *sql.Tx) error {
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
			constraint check_version_match_name check (left(mh_name, 5)::integer = mh_version),
			constraint check_version_unsigned 	check (mh_version >= 0 ),
			constraint check_filename 			check (mh_name ~ '^(\d{5})-([a-zA-Z0-9_.-]+)\.(do|dontx|r|rntx)\.sql$')
		);
	`, r.tableName)

	_, err := tx.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	return err
}

func versionedQuery(r *migrateHistoryPostgresRepository) string {
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
			mh_applied_at
		`, r.tableName)
	return query
}

func (r *migrateHistoryPostgresRepository) SaveVersioned(ctx context.Context, tx *sql.Tx, inputEntity *history.MigrateHistoryCreateInput) error {
	tag := "migrateHistoryPostgresRepository.SaveVersioned"
	query := versionedQuery(r)
	_, err := tx.ExecContext(ctx, query, inputEntity.MhVersion, inputEntity.MhName, inputEntity.MhHash)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	return nil
}

func (r *migrateHistoryPostgresRepository) SaveVersionedNoTx(ctx context.Context, conn *sql.DB, inputEntity *history.MigrateHistoryCreateInput) error {
	tag := "migrateHistoryPostgresRepository.SaveVersionedNoTx"
	query := versionedQuery(r)
	_, err := conn.ExecContext(ctx, query, inputEntity.MhVersion, inputEntity.MhName, inputEntity.MhHash)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	return nil
}

func repeatableQuery(r *migrateHistoryPostgresRepository) string {
	query := fmt.Sprintf(`		
		with updated as (
			update %s
				set mh_hash = $3,
					mh_applied_by = session_user,
					mh_applied_at = transaction_timestamp()
				where mh_name = $2
				returning id)
		insert
		into %s (mh_version, mh_name, mh_hash, mh_applied_by, mh_applied_at)
		select $1,
               $2,
			   $3,
			   session_user,
			   transaction_timestamp()
		where not exists (select 1 from updated)
		`, r.tableName, r.tableName)
	return query
}

func (r *migrateHistoryPostgresRepository) SaveRepeatable(ctx context.Context, tx *sql.Tx, inputEntity *history.MigrateHistoryCreateInput) error {
	tag := "migrateHistoryPostgresRepository.SaveRepeatable"
	query := repeatableQuery(r)
	_, err := tx.ExecContext(ctx, query, inputEntity.MhVersion, inputEntity.MhName, inputEntity.MhHash)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	return nil
}

func (r *migrateHistoryPostgresRepository) SaveRepeatableNoTx(ctx context.Context, conn *sql.DB, inputEntity *history.MigrateHistoryCreateInput) error {
	tag := "migrateHistoryPostgresRepository.SaveRepeatableNoTx"
	query := repeatableQuery(r)
	_, err := conn.ExecContext(ctx, query, inputEntity.MhVersion, inputEntity.MhName, inputEntity.MhHash)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	return nil
}

func (r *migrateHistoryPostgresRepository) ListAll(ctx context.Context, tx *sql.Tx) ([]history.MigrateHistory, error) {
	tag := "migrateHistoryPostgresRepository.ListAll"

	query := fmt.Sprintf(`		
		select
			id,
			mh_version,
			mh_name,
			mh_hash,
			mh_applied_by,
			mh_applied_at
		from %s
		order by mh_name
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
	)
	if err != nil {
		return nil, err
	}
	return &scannedEntity, nil
}
