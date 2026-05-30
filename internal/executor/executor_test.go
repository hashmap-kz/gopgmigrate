package executor_test

import (
	"errors"
	"testing"

	"github.com/hashmap-kz/gopgmigrate/v2/internal/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoTxHistoryError_RecoverySQL(t *testing.T) {
	tests := []struct {
		name        string
		err         *executor.NoTxHistoryError
		wantContain []string
	}{
		{
			name: "with description",
			err: &executor.NoTxHistoryError{
				Path:        "migrations/001_create_users.sql",
				Table:       "schema_migrations",
				Checksum:    "abc123",
				Description: "release-1.0",
			},
			wantContain: []string{
				"INSERT INTO schema_migrations",
				"migrations/001_create_users.sql",
				"no-tx",
				"abc123",
				"'release-1.0'",
			},
		},
		{
			name: "without description",
			err: &executor.NoTxHistoryError{
				Path:     "migrations/001_create_users.sql",
				Table:    "schema_migrations",
				Checksum: "abc123",
			},
			wantContain: []string{
				"INSERT INTO schema_migrations",
				"migrations/001_create_users.sql",
				"no-tx",
				"abc123",
				"NULL",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sql := tc.err.RecoverySQL()
			for _, want := range tc.wantContain {
				assert.Contains(t, sql, want)
			}
		})
	}
}

func TestNoTxHistoryError_RecoverySQL_EmptyDescriptionIsNull(t *testing.T) {
	e := &executor.NoTxHistoryError{
		Path: "a.sql", Table: "t", Checksum: "x", Description: "",
	}
	assert.Contains(t, e.RecoverySQL(), "NULL")
	assert.NotContains(t, e.RecoverySQL(), "''")
}

func TestNoTxHistoryError_Error_ContainsKeyInfo(t *testing.T) {
	cause := errors.New("connection reset by peer")
	e := &executor.NoTxHistoryError{
		Path:     "migrations/001.sql",
		Table:    "schema_migrations",
		Checksum: "abc",
		Cause:    cause,
	}
	msg := e.Error()
	assert.Contains(t, msg, "migrations/001.sql")
	assert.Contains(t, msg, "connection reset by peer")
}

func TestNoTxHistoryError_Unwrap(t *testing.T) {
	cause := errors.New("some db error")
	e := &executor.NoTxHistoryError{Cause: cause}
	assert.Equal(t, cause, errors.Unwrap(e))
	require.ErrorIs(t, e, cause)
}
