//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hashmap-kz/gopgmigrate/v2/pkg/migrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSafety_ChecksumMismatchRejected(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")

	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
	})
	cfg := migrator.Config{ManifestPath: manifest, Table: histTable}

	m1, err := migrator.NewWithDSN(pg.ConnStr, cfg)
	require.NoError(t, err)
	defer m1.Close()
	require.NoError(t, m1.Run(context.Background()))

	// modify the already-applied versioned migration
	dir.Add(t, "001_create_users.sql", "create table users (id bigint primary key);")

	m2, err := migrator.NewWithDSN(pg.ConnStr, cfg)
	require.NoError(t, err)
	defer m2.Close()
	err = m2.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
}

func TestSafety_AdvisoryLockBlocksConcurrent(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_slow.sql", "select pg_sleep(2);")

	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_slow.sql"}},
	})

	errCh := make(chan error, 2)
	run := func() {
		m, err := migrator.NewWithDSN(pg.ConnStr, migrator.Config{
			ManifestPath: manifest,
			Table:        histTable,
		})
		if err != nil {
			errCh <- err
			return
		}
		defer m.Close()
		errCh <- m.Run(context.Background())
	}

	go run()
	time.Sleep(200 * time.Millisecond) // let first goroutine acquire the advisory lock
	go run()

	err1 := <-errCh
	err2 := <-errCh

	var lockErrs int
	for _, e := range []error{err1, err2} {
		if e != nil && strings.Contains(e.Error(), "lock") {
			lockErrs++
		}
	}
	assert.Equal(t, 1, lockErrs, "exactly one run should fail with a lock error")
}

func TestStatus_ShowsAppliedAndPending(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	dir.Add(t, "002_add_email.sql", "alter table users add column email text;")

	// apply only the first migration
	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
	})
	m1, err := migrator.NewWithDSN(pg.ConnStr, migrator.Config{ManifestPath: manifest, Table: histTable})
	require.NoError(t, err)
	defer m1.Close()
	require.NoError(t, m1.Run(context.Background()))

	// add the second migration and check status
	manifest = dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
		{Files: []string{"002_add_email.sql"}},
	})
	m2, err := migrator.NewWithDSN(pg.ConnStr, migrator.Config{ManifestPath: manifest, Table: histTable})
	require.NoError(t, err)
	defer m2.Close()

	statuses, err := m2.Status(context.Background())
	require.NoError(t, err)
	require.Len(t, statuses, 2)
	assert.True(t, statuses[0].Applied, "first migration should be applied")
	assert.False(t, statuses[1].Applied, "second migration should be pending")
}

func TestStatus_ShowsChecksumMismatch(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")

	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
	})
	cfg := migrator.Config{ManifestPath: manifest, Table: histTable}

	m1, err := migrator.NewWithDSN(pg.ConnStr, cfg)
	require.NoError(t, err)
	defer m1.Close()
	require.NoError(t, m1.Run(context.Background()))

	// modify the applied file
	dir.Add(t, "001_create_users.sql", "create table users (id bigint primary key);")

	m2, err := migrator.NewWithDSN(pg.ConnStr, cfg)
	require.NoError(t, err)
	defer m2.Close()

	statuses, err := m2.Status(context.Background())
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.Contains(t, statuses[0].Kind, "CHECKSUM MISMATCH")
}
