package impl

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"

	"github.com/google/uuid"

	"gopgmigrate/internal/mode"
	"gopgmigrate/internal/version"

	"gopgmigrate/internal/dbms"
	"gopgmigrate/internal/history"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type migrateHistoryClickhouseRepository struct {
	tableName string
}

var _ history.MigrateHistoryRepository = &migrateHistoryClickhouseRepository{}

func NewMigrateHistoryClickhouseRepository(_ context.Context, tableName string) history.MigrateHistoryRepository {
	return &migrateHistoryClickhouseRepository{
		tableName: tableName,
	}
}

// TODO: simplify
func (r *migrateHistoryClickhouseRepository) CreateHistoryTable(ctx context.Context, tx dbms.Transaction) error {
	tag := "migrateHistoryClickhouseRepository.CreateHistoryTable"

	query := fmt.Sprintf(`
		create table if not exists %s
		(
			mh_version       Int64  not null,
			mh_name          String not null,
			mh_hash          String not null,
			mh_applied_by    String                   default currentUser(),
			mh_applied_at    DateTime64(3, 'UTC')     default now64(3),
			mh_iter_id		 UUID not null,
			mh_version_check Int64 MATERIALIZED       toInt64(left(mh_name, 5)),
			constraint       check_filename           check mh_name REGEXP '^(\d{5})-(.*)(?:\.ntx)?\.(do|r)\.sql$',
			constraint       check_version_unsigned   check mh_version >= 0,
			constraint       check_version_match_name check mh_version_check = mh_version
		)
		ENGINE = MergeTree()
		ORDER BY (mh_version);
  `, r.tableName)

	_, err := tx.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	return err
}

func (r *migrateHistoryClickhouseRepository) SaveVersioned(ctx context.Context, tx dbms.Transaction, inputEntity *history.MigrateHistoryCreateInput) error {
	tag := "migrateHistoryClickhouseRepository.SaveVersioned"
	query := fmt.Sprintf(`		
		insert into %s (mh_version, mh_name, mh_hash, mh_iter_id, mh_applied_by, mh_applied_at)
		values ($1, $2, $3, $4, currentUser(), now64(3));
		`, r.tableName)
	_, err := tx.ExecContext(ctx, query,
		inputEntity.MhVersion,
		inputEntity.MhName,
		inputEntity.MhHash,
		inputEntity.MhIterID,
	)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	return nil
}

// TODO: simplify
func (r *migrateHistoryClickhouseRepository) SaveRepeatable(ctx context.Context, tx dbms.Transaction, inputEntity *history.MigrateHistoryCreateInput) error {
	tag := "migrateHistoryClickhouseRepository.SaveRepeatable"

	// upsert

	exists, err := r.VersionExists(ctx, tx, inputEntity.MhVersion)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}

	// insert
	if !exists {
		err := r.SaveVersioned(ctx, tx, inputEntity)
		if err != nil {
			return fmt.Errorf("%s: %w", tag, err)
		}
		return nil
	}

	// update
	query := fmt.Sprintf(`    
		alter table %s
		update
			mh_hash 		= $2,
			mh_applied_at 	= now64(3),
			mh_iter_id    	= $3
		where
			mh_version = $1
			settings mutations_sync = 2
    `, r.tableName)
	_, err = tx.ExecContext(ctx, query,
		inputEntity.MhVersion,
		inputEntity.MhHash,
		inputEntity.MhIterID,
	)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	return nil
}

func (r *migrateHistoryClickhouseRepository) ListAll(ctx context.Context, tx dbms.Transaction) ([]history.MigrateHistory, error) {
	tag := "migrateHistoryClickhouseRepository.ListAll"

	query := fmt.Sprintf(`		
		select
			mh_version,
			mh_name,
			mh_hash,
			mh_applied_by,
			mh_applied_at,
			mh_iter_id
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
		scannedEntity, err := scanFullRowCh(rows)
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

func (r *migrateHistoryClickhouseRepository) DeleteVersion(ctx context.Context, tx dbms.Transaction, v int64) error {
	tag := "migrateHistoryClickhouseRepository.DeleteVersion"
	query := fmt.Sprintf(`
		delete from %s
		where mh_version = $1
	`, r.tableName)
	_, err := tx.ExecContext(ctx, query, v)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	return nil
}

func (r *migrateHistoryClickhouseRepository) VersionExists(ctx context.Context, tx dbms.Transaction, v int64) (bool, error) {
	var exists bool
	query := fmt.Sprintf("select exists (select 1 from %s where mh_version = $1);", r.tableName)

	err := tx.QueryRowContext(context.Background(), query, v).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// utils

func (r *migrateHistoryClickhouseRepository) GetNoTxPatterns() map[string]*regexp.Regexp {
	// of course there are not :)
	return map[string]*regexp.Regexp{}
}

// locks

// AcquireMigrationLock ensures only one migration process runs at a time
func (r *migrateHistoryClickhouseRepository) AcquireMigrationLock(ctx context.Context, db dbms.Transaction) (bool, error) {
	// TODO:
	return true, nil
}

// ReleaseMigrationLock releases the advisory lock
func (r *migrateHistoryClickhouseRepository) ReleaseMigrationLock(ctx context.Context, db dbms.Transaction) error {
	// TODO:
	return nil
}

// scan utils

func scanFullRowCh(row *sql.Rows) (*history.MigrateHistory, error) {
	var scannedEntity history.MigrateHistory
	err := row.Scan(
		&scannedEntity.MhVersion,
		&scannedEntity.MhName,
		&scannedEntity.MhHash,
		&scannedEntity.MhAppliedBy,
		&scannedEntity.MhAppliedAt,
		&scannedEntity.MhIterID,
	)
	if err != nil {
		return nil, err
	}
	return &scannedEntity, nil
}

// migration

func (r *migrateHistoryClickhouseRepository) RunMigrationsPlainMode(
	ctx context.Context,
	db *sql.DB,
	pendingMigrations []version.MigrationFile,
	directionDo bool,
) error {
	iterId := uuid.New()
	err := history.MigrateListOfFilesNoTx(ctx, db, pendingMigrations, r, directionDo, iterId)
	if err != nil {
		return err
	}
	return nil
}

func (r *migrateHistoryClickhouseRepository) RunMigrationsMixedMode(
	ctx context.Context,
	db *sql.DB,
	groupEntries []mode.GroupEntry,
	directionDo bool,
) error {
	iterId := uuid.New()
	pendingMigrations := []version.MigrationFile{}
	for _, ge := range groupEntries {
		pendingMigrations = append(pendingMigrations, ge.Files...)
	}
	err := history.MigrateListOfFilesNoTx(ctx, db, pendingMigrations, r, directionDo, iterId)
	if err != nil {
		return err
	}
	return nil
}

func (r *migrateHistoryClickhouseRepository) RunMigrationsGroupMode(
	ctx context.Context,
	db *sql.DB,
	groupEntry mode.GroupEntry,
	directionDo bool,
) error {
	iterId := uuid.New()
	err := history.MigrateListOfFilesNoTx(ctx, db, groupEntry.Files, r, directionDo, iterId)
	if err != nil {
		return err
	}
	return nil
}
