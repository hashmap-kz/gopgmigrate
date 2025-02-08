package migrate_history

import "time"

// MigrateHistory tracks executed migrations, ensuring version control and repeatable migrations.
type MigrateHistory struct {
	// Auto-incrementing primary key.
	ID int `json:"id" db:"id"`

	// Version number of the migration (bigint). Used for versioned migrations.
	MhVersion *int64 `json:"mh_version" db:"mh_version"`

	// Migration type: schema, data, or repeatable.
	MhMode string `json:"mh_mode" db:"mh_mode"`

	// Name of the migration file applied.
	MhName string `json:"mh_name" db:"mh_name"`

	// SHA256 hash of the migration script to detect changes in repeatable migrations.
	MhHash string `json:"mh_hash" db:"mh_hash"`

	// User who executed the migration.
	MhAppliedBy string `json:"mh_applied_by" db:"mh_applied_by"`

	// Timestamp when the migration was applied.
	MhAppliedAt time.Time `json:"mh_applied_at" db:"mh_applied_at"`
}
