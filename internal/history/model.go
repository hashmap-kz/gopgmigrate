package history

import "time"

type MigrateHistoryCreateInput struct {
	MhVersion int64
	MhName    string
	MhHash    string
	MhIterID  string
}

// MigrateHistory tracks executed migrations, ensuring version control and repeatable migrations.
type MigrateHistory struct {
	// Auto-incrementing primary key.
	ID int `json:"id" db:"id"`

	// Version number of the migration (bigint). Used for versioned migrations.
	MhVersion int64 `json:"mh_version" db:"mh_version"`

	// Name of the migration file applied.
	MhName string `json:"mh_name" db:"mh_name"`

	// SHA256 hash of the migration script to detect changes in repeatable migrations.
	MhHash string `json:"mh_hash" db:"mh_hash"`

	// User who executed the migration.
	MhAppliedBy string `json:"mh_applied_by" db:"mh_applied_by"`

	// Timestamp when the migration was applied.
	MhAppliedAt time.Time `json:"mh_applied_at" db:"mh_applied_at"`

	// Current transaction ID, for debug purpose, optional, may be empty
	MhTxid string `json:"mh_txid" db:"mh_txid"`

	// Iteration ID, for debug purpose, unique migration step
	MhIterID string `json:"mh_iter_id" db:"mh_iter_id"`
}
