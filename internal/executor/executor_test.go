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
			name: "basic",
			err: &executor.NoTxHistoryError{
				MigrationID: 1,
				Path:        "0000001-create-users.notx.sql",
				Table:       "schema_migrations",
				Checksum:    "abc123",
			},
			wantContain: []string{
				"INSERT INTO schema_migrations",
				"0000001-create-users.notx.sql",
				"no-tx",
				"abc123",
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

func TestNoTxHistoryError_RecoverySQL_ContainsMigrationID(t *testing.T) {
	e := &executor.NoTxHistoryError{
		MigrationID: 42, Path: "0000042-vacuum.notx.sql", Table: "t", Checksum: "x",
	}
	assert.Contains(t, e.RecoverySQL(), "42")
}

func TestNoTxHistoryError_Error_ContainsKeyInfo(t *testing.T) {
	cause := errors.New("connection reset by peer")
	e := &executor.NoTxHistoryError{
		MigrationID: 1,
		Path:        "0000001-vacuum.notx.sql",
		Table:       "schema_migrations",
		Checksum:    "abc",
		Cause:       cause,
	}
	msg := e.Error()
	assert.Contains(t, msg, "0000001-vacuum.notx.sql")
	assert.Contains(t, msg, "connection reset by peer")
}

func TestNoTxHistoryError_Unwrap(t *testing.T) {
	cause := errors.New("some db error")
	e := &executor.NoTxHistoryError{MigrationID: 1, Cause: cause}
	assert.Equal(t, cause, errors.Unwrap(e))
	require.ErrorIs(t, e, cause)
}
