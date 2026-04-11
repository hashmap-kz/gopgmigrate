package history

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Migration Lock Key (must be unique per application)
const migrationLockKey = 123456

type Transaction interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

type MigrateHistoryCreateInput struct {
	Version int64
	Name    string
	Hash    string
}

// MigrateHistory tracks executed migrations, ensuring version control and repeatable migrations.
type MigrateHistory struct {
	// Auto-incrementing primary key.
	ID int `json:"id" db:"id"`

	// Version number of the migration (bigint). Used for versioned migrations.
	Version int64 `json:"mh_version" db:"mh_version"`

	// Name of the migration file applied.
	Name string `json:"mh_name" db:"mh_name"`

	// SHA256 hash of the migration script to detect changes in repeatable migrations.
	Hash string `json:"mh_hash" db:"mh_hash"`

	// User who executed the migration.
	AppliedBy string `json:"mh_applied_by" db:"mh_applied_by"`

	// Timestamp when the migration was applied.
	AppliedAt time.Time `json:"mh_applied_at" db:"mh_applied_at"`

	// Current transaction ID, for debug purpose, optional, may be empty
	TxID string `json:"mh_txid" db:"mh_txid"`
}

type MigrateHistoryRepository interface {
	CreateHistoryTable(ctx context.Context, tx Transaction) error

	SaveVersioned(ctx context.Context, tx Transaction, inputEntity *MigrateHistoryCreateInput) error
	SaveRepeatable(ctx context.Context, tx Transaction, inputEntity *MigrateHistoryCreateInput) error
	ListAll(ctx context.Context, tx Transaction) ([]MigrateHistory, error)
	DeleteVersion(ctx context.Context, tx Transaction, v int64) error

	AcquireMigrationLock(ctx context.Context, db Transaction) (bool, error)
	ReleaseMigrationLock(ctx context.Context, db Transaction) error

	GetNoTxPatterns() map[string]*regexp.Regexp
}

type repo struct {
	tableName string
}

var _ MigrateHistoryRepository = &repo{}

func NewMigrateHistoryPostgresRepository(_ context.Context, tableName string) MigrateHistoryRepository {
	return &repo{
		tableName: tableName,
	}
}

func (r *repo) CreateHistoryTable(ctx context.Context, tx Transaction) error {
	tag := "repo.CreateHistoryTable"

	query := fmt.Sprintf(`
		create table if not exists %s
		(
		  id            int generated always as identity primary key,
		  mh_version    bigint unique not null,
		  mh_name       text unique   not null,
		  mh_hash       text          not null,
		  mh_applied_by name          not null default session_user,
		  mh_applied_at timestamptz   not null default transaction_timestamp(),
		  mh_txid     text            not null default pg_current_xact_id()::text
		);
  `, r.tableName)

	_, err := tx.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	return err
}

func (r *repo) SaveVersioned(ctx context.Context, tx Transaction, inputEntity *MigrateHistoryCreateInput) error {
	tag := "repo.SaveVersioned"
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
	_, err := tx.ExecContext(ctx, query,
		inputEntity.Version,
		inputEntity.Name,
		inputEntity.Hash,
	)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	return nil
}

func (r *repo) SaveRepeatable(
	ctx context.Context,
	tx Transaction,
	inputEntity *MigrateHistoryCreateInput,
) error {
	tag := "repo.SaveRepeatable"
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
	_, err := tx.ExecContext(ctx, query,
		inputEntity.Version,
		inputEntity.Name,
		inputEntity.Hash,
	)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	return nil
}

func (r *repo) ListAll(ctx context.Context, tx Transaction) ([]MigrateHistory, error) {
	tag := "repo.ListAll"

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

func (r *repo) DeleteVersion(ctx context.Context, tx Transaction, v int64) error {
	tag := "repo.DeleteVersion"
	query := fmt.Sprintf(`
		delete from only %s
		where mh_version = $1
	`, r.tableName)
	res, err := tx.ExecContext(ctx, query, v)
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", tag, err)
	}
	if rowsAffected <= 0 {
		return fmt.Errorf("%s: no rows were deleted for version: %d", tag, v)
	}
	return nil
}

// utils

func (r *repo) GetNoTxPatterns() map[string]*regexp.Regexp {
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

// locks

// AcquireMigrationLock ensures only one migration process runs at a time
func (r *repo) AcquireMigrationLock(ctx context.Context, db Transaction) (bool, error) {
	var acquired bool
	err := db.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", migrationLockKey).Scan(&acquired)
	return acquired, err
}

// ReleaseMigrationLock releases the advisory lock
func (r *repo) ReleaseMigrationLock(ctx context.Context, db Transaction) error {
	_, err := db.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", migrationLockKey)
	return err
}

// scan utils

func scanFullRow(row *sql.Rows) (*MigrateHistory, error) {
	var scannedEntity MigrateHistory
	err := row.Scan(
		&scannedEntity.ID,
		&scannedEntity.Version,
		&scannedEntity.Name,
		&scannedEntity.Hash,
		&scannedEntity.AppliedBy,
		&scannedEntity.AppliedAt,
		&scannedEntity.TxID,
	)
	if err != nil {
		return nil, err
	}
	return &scannedEntity, nil
}
