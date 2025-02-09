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

func (r *migrateHistoryRepository) Save(ctx context.Context, inputEntity *MigrateHistoryVersionedCreateInput) (*MigrateHistory, error) {
	tag := "migrateHistoryRepository.Save"

	query := `		
		insert into public.migrate_history (
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
		`

	row := r.db.QueryRow(ctx, query,
		inputEntity.MhVersion,
		inputEntity.MhName,
		inputEntity.MhHash,
	)

	scannedEntity, err := scanFullRow(row)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tag, err)
	}
	return scannedEntity, nil
}

func (r *migrateHistoryRepository) SaveOrUpdate(ctx context.Context, inputEntity *MigrateHistoryRepeatableCreateInput) (*MigrateHistory, error) {
	existsByName, err := r.ExistsByName(ctx, inputEntity.MhName)
	if err != nil {
		return nil, err
	}
	if existsByName {
		return r.UpdateByName(ctx, inputEntity.MhHash, inputEntity.MhName)
	}
	return r.Save(ctx, &MigrateHistoryVersionedCreateInput{
		MhName: inputEntity.MhName,
		MhHash: inputEntity.MhHash,
	})
}

func (r *migrateHistoryRepository) UpdateByName(ctx context.Context, newHash string, name string) (*MigrateHistory, error) {
	tag := "migrateHistoryRepository.UpdateByName"

	// update is available ONLY for repeatable migrations
	// and we're able to update ONLY the hash field
	query := `		
		update public.migrate_history
		set
			mh_hash       = $2,
			mh_applied_by = session_user,
			mh_applied_at = transaction_timestamp()
		where mh_name = $1
		returning 
			id,
			mh_version,
			mh_name,
			mh_hash,
			mh_applied_by,
			mh_applied_at
		`

	row := r.db.QueryRow(ctx, query,
		name,
		newHash,
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
			mh_name,
			mh_hash,
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

func (r *migrateHistoryRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	var exists bool
	query := `select exists(select 1 from public.migrate_history where mh_name = $1)`
	err := r.db.QueryRow(context.Background(), query, name).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *migrateHistoryRepository) FindByName(ctx context.Context, name string) (*MigrateHistory, error) {
	tag := "migrateHistoryRepository.FindByNameMode"

	query := `		
		select
			id,
			mh_version,
			mh_name,
			mh_hash,
			mh_applied_by,
			mh_applied_at
		from public.migrate_history
		where 
			mh_name = $1
		`

	row := r.db.QueryRow(ctx, query, name)

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
			mh_name,
			mh_hash,
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

func (r *migrateHistoryRepository) GetAppliedNames(ctx context.Context) (map[string]bool, error) {
	all, err := r.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	applied := map[string]bool{}
	for _, e := range all {
		applied[e.MhName] = true
	}
	return applied, nil
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
