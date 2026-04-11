package history

import "time"

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
