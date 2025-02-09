package impl

import (
	"context"
	"database/sql"
	"fmt"

	"gopgmigrate/internal/migrate_history"
)

type migrateHistoryPostgresRepository struct {
	tableName string
	db        *sql.DB
}

var _ migrate_history.MigrateHistoryRepository = &migrateHistoryPostgresRepository{}

func NewMigrateHistoryPostgresRepository(_ context.Context, db *sql.DB, tableName string) migrate_history.MigrateHistoryRepository {
	return &migrateHistoryPostgresRepository{
		db:        db,
		tableName: tableName,
	}
}

func (r *migrateHistoryPostgresRepository) CreateHistoryTable(ctx context.Context) error {
	tag := "migrateHistoryPostgresRepository.CreateHistoryTable"

	query := fmt.Sprintf(`
		create table if not exists %s
		(
			id            int 		  generated always as identity primary key,
			mh_version    bigint 	  unique,
			mh_name       text        unique not null,
			mh_hash       text        not null,
			mh_applied_by name        not null default session_user,
			mh_applied_at timestamptz not null default transaction_timestamp()
		);
	`, r.tableName)

	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	return err
}

func (r *migrateHistoryPostgresRepository) SaveVersioned(ctx context.Context, inputEntity *migrate_history.MigrateHistoryVersionedCreateInput) error {
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
			mh_applied_at
		`, r.tableName)

	_, err := r.db.ExecContext(ctx, query, inputEntity.MhVersion, inputEntity.MhName, inputEntity.MhHash)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	return nil
}

func (r *migrateHistoryPostgresRepository) SaveRepeatable(ctx context.Context, inputEntity *migrate_history.MigrateHistoryRepeatableCreateInput) error {
	tag := "migrateHistoryPostgresRepository.SaveRepeatable"

	query := fmt.Sprintf(`		
		with updated as (
			update %s
				set mh_hash = $2,
					mh_applied_by = session_user,
					mh_applied_at = transaction_timestamp()
				where mh_name = $1
				returning id)
		insert
		into %s (mh_name, mh_hash, mh_applied_by, mh_applied_at)
		select $1,
			   $2,
			   session_user,
			   transaction_timestamp()
		where not exists (select 1 from updated)
		`, r.tableName, r.tableName)

	_, err := r.db.ExecContext(ctx, query, inputEntity.MhName, inputEntity.MhHash)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	return nil
}

func (r *migrateHistoryPostgresRepository) ListAll(ctx context.Context) ([]migrate_history.MigrateHistory, error) {
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

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tag, err)
	}
	defer rows.Close()

	var scannedEntities []migrate_history.MigrateHistory
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
func scanFullRow(row *sql.Rows) (*migrate_history.MigrateHistory, error) {
	var scannedEntity migrate_history.MigrateHistory
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
