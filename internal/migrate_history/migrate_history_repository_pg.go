package migrate_history

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type migrateHistoryRepository struct {
	db *pgx.Conn
}

var _ MigrateHistoryRepository = &migrateHistoryRepository{}

func NewMigrateHistoryRepository(_ context.Context, db *pgx.Conn) MigrateHistoryRepository {
	return &migrateHistoryRepository{
		db: db,
	}
}

func (r *migrateHistoryRepository) Save(ctx context.Context, inputEntity *MigrateHistory) (*MigrateHistory, error) {
	tag := "migrateHistoryRepository.Save"

	query := `		
		insert into public.migrate_history (
			mh_version,
			mh_mode,
			mh_name,
			mh_hash,
			mh_txid,
			mh_applied_by,
			mh_applied_at
		)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning
			id,
			mh_version,
			mh_mode,
			mh_name,
			mh_hash,
			mh_txid,
			mh_applied_by,
			mh_applied_at
		`

	row := r.db.QueryRow(ctx, query,
		inputEntity.MhVersion,
		inputEntity.MhMode,
		inputEntity.MhName,
		inputEntity.MhHash,
		inputEntity.MhTxid,
		inputEntity.MhAppliedBy,
		inputEntity.MhAppliedAt,
	)

	scannedEntity, err := scanFullRow(row)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tag, err)
	}
	return scannedEntity, nil
}

func (r *migrateHistoryRepository) UpdateByID(ctx context.Context, inputEntity *MigrateHistory, pkID int) (*MigrateHistory, error) {
	tag := "migrateHistoryRepository.UpdateByID"

	query := `		
		update public.migrate_history
		set 
			mh_version    = coalesce(nullif($2, 0::int8), mh_version),
			mh_mode       = coalesce(nullif($3, ''), mh_mode),
			mh_name       = coalesce(nullif($4, ''), mh_name),
			mh_hash       = coalesce(nullif($5, ''), mh_hash),
			mh_txid       = coalesce(nullif($6, '0'::xid8), mh_txid),
			mh_applied_by = coalesce(nullif($7, ''), mh_applied_by),
			mh_applied_at = coalesce(nullif($8, '0001-01-01 00:00:00'::timestamp), mh_applied_at)
		where id = $1
		returning 
			id,
			mh_version,
			mh_mode,
			mh_name,
			mh_hash,
			mh_txid,
			mh_applied_by,
			mh_applied_at
		`

	row := r.db.QueryRow(ctx, query,
		pkID,
		inputEntity.MhVersion,
		inputEntity.MhMode,
		inputEntity.MhName,
		inputEntity.MhHash,
		inputEntity.MhTxid,
		inputEntity.MhAppliedBy,
		inputEntity.MhAppliedAt,
	)

	scannedEntity, err := scanFullRow(row)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tag, err)
	}
	return scannedEntity, nil
}

func (r *migrateHistoryRepository) DeleteByID(ctx context.Context, pkID int) error {
	tag := "migrateHistoryRepository.DeleteByID"

	query := `		
		delete from only public.migrate_history
		where id = $1
		`

	cmdTag, err := r.db.Exec(ctx, query, pkID)
	if err != nil || cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("%s. no rows were deleted: %w", tag, err)
	}
	return nil
}

func (r *migrateHistoryRepository) FindByID(ctx context.Context, pkID int) (*MigrateHistory, error) {
	tag := "migrateHistoryRepository.FindByID"

	query := `		
		select
			id,
			mh_version,
			mh_mode,
			mh_name,
			mh_hash,
			mh_txid,
			mh_applied_by,
			mh_applied_at
		from public.migrate_history
		where id = $1
		order by id
		`

	row := r.db.QueryRow(ctx, query, pkID)

	scannedEntity, err := scanFullRow(row)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tag, err)
	}
	return scannedEntity, nil
}

func (r *migrateHistoryRepository) FindAll(ctx context.Context) ([]MigrateHistory, error) {
	tag := "migrateHistoryRepository.FindAll"

	query := `		
		select
			id,
			mh_version,
			mh_mode,
			mh_name,
			mh_hash,
			mh_txid,
			mh_applied_by,
			mh_applied_at
		from public.migrate_history
		order by id
		`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tag, err)
	}
	defer rows.Close()

	var scannedEntities []MigrateHistory
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
func scanFullRow(row pgx.Row) (*MigrateHistory, error) {
	var scannedEntity MigrateHistory
	err := row.Scan(
		&scannedEntity.ID,
		&scannedEntity.MhVersion,
		&scannedEntity.MhMode,
		&scannedEntity.MhName,
		&scannedEntity.MhHash,
		&scannedEntity.MhTxid,
		&scannedEntity.MhAppliedBy,
		&scannedEntity.MhAppliedAt,
	)
	if err != nil {
		return nil, err
	}
	return &scannedEntity, nil
}
