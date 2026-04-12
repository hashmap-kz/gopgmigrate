package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopgmigrate/internal/migration"
)

// integration/safety_test.go
func TestSafety_HashMismatchRejected(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "0000001-create-users.up.sql",
		"create table users (id int primary key);")

	opts := &migration.ApplyOpts{
		MigrationDir:     dir.Root,
		ConnStr:          pg.ConnStr,
		HistoryTableName: "public.migrate_history",
	}

	require.NoError(t, migration.RunMigrationsUp(context.Background(), opts))

	// modify an already-applied versioned migration
	dir.Add(t, "0000001-create-users.up.sql",
		"create table users (id bigint primary key);")

	err := migration.RunMigrationsUp(context.Background(), opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hash mismatch")
}

func TestSafety_AdvisoryLockBlocksConcurrent(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	// a slow migration — gives us time to attempt a second run
	dir.Add(t, "0000001-slow.up.sql", "select pg_sleep(2);")

	opts := &migration.ApplyOpts{
		MigrationDir:     dir.Root,
		ConnStr:          pg.ConnStr,
		HistoryTableName: "public.migrate_history",
	}

	errCh := make(chan error, 2)

	go func() { errCh <- migration.RunMigrationsUp(context.Background(), opts) }()
	time.Sleep(200 * time.Millisecond) // let first acquire the lock
	go func() { errCh <- migration.RunMigrationsUp(context.Background(), opts) }()

	err1 := <-errCh
	err2 := <-errCh

	// one must succeed, one must fail with lock error
	errs := []error{err1, err2}
	lockErr := 0
	for _, e := range errs {
		if e != nil && strings.Contains(e.Error(), "lock") {
			lockErr++
		}
	}
	assert.Equal(t, 1, lockErr)
}

func TestSafety_StrayFileRejected(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "0000001-create-users.up.sql",
		"create table users (id int primary key);")
	dir.Add(t, "not-a-migration.sql", "select 1;") // stray

	err := migration.RunMigrationsUp(context.Background(), &migration.ApplyOpts{
		MigrationDir:     dir.Root,
		ConnStr:          pg.ConnStr,
		HistoryTableName: "public.migrate_history",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stray")
}
