package executor_test

import (
	"errors"
	"testing"

	"github.com/hashmap-kz/gopgmigrate/v2/internal/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoTxHistoryError_RecoverySQL(t *testing.T) {
	e := &executor.NoTxHistoryError{
		MigrationID: 1,
		Table:       "schema_migrations",
		Checksum:    "abc123",
	}
	sql := e.RecoverySQL()
	assert.Contains(t, sql, "INSERT INTO schema_migrations")
	assert.Contains(t, sql, "no-tx")
	assert.Contains(t, sql, "abc123")
	assert.NotContains(t, sql, "path")
}

func TestNoTxHistoryError_RecoverySQL_ContainsMigrationID(t *testing.T) {
	e := &executor.NoTxHistoryError{
		MigrationID: 42, Table: "t", Checksum: "x",
	}
	assert.Contains(t, e.RecoverySQL(), "42")
}

func TestNoTxHistoryError_Error_ContainsKeyInfo(t *testing.T) {
	cause := errors.New("connection reset by peer")
	e := &executor.NoTxHistoryError{
		MigrationID: 1,
		Table:       "schema_migrations",
		Checksum:    "abc",
		Cause:       cause,
	}
	msg := e.Error()
	assert.Contains(t, msg, "1")
	assert.Contains(t, msg, "connection reset by peer")
}

func TestNoTxHistoryError_Unwrap(t *testing.T) {
	cause := errors.New("some db error")
	e := &executor.NoTxHistoryError{MigrationID: 1, Cause: cause}
	assert.Equal(t, cause, errors.Unwrap(e))
	require.ErrorIs(t, e, cause)
}
