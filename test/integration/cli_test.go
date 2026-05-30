//go:build integration

package integration

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var cliBinary string

func TestMain(m *testing.M) {
	bin := filepath.Join("bin", "gopgmigrate")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if _, err := os.Stat(bin); err != nil {
		fmt.Fprintf(os.Stderr, "cli binary not found at %s - run make test-integration\n", bin)
		os.Exit(1)
	}
	cliBinary = bin
	os.Exit(m.Run())
}

type cliResult struct {
	Stdout string
	Stderr string
	Code   int
}

func runCLI(t *testing.T, args ...string) cliResult {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	cmd := exec.Command(cliBinary, args...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("exec: %v", err)
		}
	}
	return cliResult{
		Stdout: outBuf.String(),
		Stderr: errBuf.String(),
		Code:   code,
	}
}

func TestCLI_Up(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
	})

	res := runCLI(t, "up", "--dsn", pg.ConnStr, "--manifest", manifest, "--table", histTable)
	assert.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
	assert.True(t, TableExists(t, pg.DB, "public", "users"))
	assert.Len(t, QueryHistory(t, pg.DB, histTable), 1)
}

func TestCLI_Up_Idempotent(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
	})
	args := []string{"up", "--dsn", pg.ConnStr, "--manifest", manifest, "--table", histTable}

	res1 := runCLI(t, args...)
	assert.Equal(t, 0, res1.Code, "first run stderr: %s", res1.Stderr)

	res2 := runCLI(t, args...)
	assert.Equal(t, 0, res2.Code, "second run stderr: %s", res2.Stderr)

	assert.Len(t, QueryHistory(t, pg.DB, histTable), 1)
}

func TestCLI_Up_DryRun(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
	})

	res := runCLI(t, "up", "--dsn", pg.ConnStr, "--manifest", manifest, "--table", histTable, "--dry-run")
	assert.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
	assert.False(t, TableExists(t, pg.DB, "public", "users"))
	assert.Empty(t, QueryHistory(t, pg.DB, histTable))
}

func TestCLI_Up_ChecksumMismatchFails(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
	})
	args := []string{"up", "--dsn", pg.ConnStr, "--manifest", manifest, "--table", histTable}

	res1 := runCLI(t, args...)
	require.Equal(t, 0, res1.Code, "stderr: %s", res1.Stderr)

	dir.Add(t, "001_create_users.sql", "create table users (id bigint primary key);")

	res2 := runCLI(t, args...)
	assert.NotEqual(t, 0, res2.Code)
}

func TestCLI_Status(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	dir.Add(t, "002_add_email.sql", "alter table users add column email text;")

	// apply only the first migration
	partialManifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
	})
	res := runCLI(t, "up", "--dsn", pg.ConnStr, "--manifest", partialManifest, "--table", histTable)
	require.Equal(t, 0, res.Code, "up stderr: %s", res.Stderr)

	// expand manifest and check status
	fullManifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
		{Files: []string{"002_add_email.sql"}},
	})
	res = runCLI(t, "status", "--dsn", pg.ConnStr, "--manifest", fullManifest, "--table", histTable)
	assert.Equal(t, 0, res.Code, "status stderr: %s", res.Stderr)
	assert.Contains(t, res.Stdout, "APPLIED")
	assert.Contains(t, res.Stdout, "yes")
	assert.Contains(t, res.Stdout, "no")
}

func TestCLI_Validate_OK(t *testing.T) {
	t.Parallel()
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
	})

	res := runCLI(t, "validate", "--manifest", manifest)
	assert.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
	assert.Contains(t, res.Stdout, "manifest OK")
}

func TestCLI_Validate_MissingFile(t *testing.T) {
	t.Parallel()
	dir := NewMigrationDir(t)

	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"nonexistent.sql"}},
	})

	res := runCLI(t, "validate", "--manifest", manifest)
	assert.NotEqual(t, 0, res.Code)
}

func TestCLI_Up_MissingRequiredDSN(t *testing.T) {
	t.Parallel()
	dir := NewMigrationDir(t)

	dir.Add(t, "001.sql", "select 1;")
	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001.sql"}},
	})

	res := runCLI(t, "up", "--manifest", manifest)
	assert.NotEqual(t, 0, res.Code)
}
