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

func TestCLI_Apply(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
	})

	res := runCLI(t, "apply", "--dsn", pg.ConnStr, "--manifest", manifest, "--table", histTable)
	assert.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
	assert.True(t, TableExists(t, pg.DB, "public", "users"))
	assert.Len(t, QueryHistory(t, pg.DB, histTable), 1)
}

func TestCLI_Apply_Idempotent(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
	})
	args := []string{"apply", "--dsn", pg.ConnStr, "--manifest", manifest, "--table", histTable}

	res1 := runCLI(t, args...)
	assert.Equal(t, 0, res1.Code, "first run stderr: %s", res1.Stderr)

	res2 := runCLI(t, args...)
	assert.Equal(t, 0, res2.Code, "second run stderr: %s", res2.Stderr)

	assert.Len(t, QueryHistory(t, pg.DB, histTable), 1)
}

func TestCLI_Apply_ChecksumMismatchFails(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
	})
	args := []string{"apply", "--dsn", pg.ConnStr, "--manifest", manifest, "--table", histTable}

	res1 := runCLI(t, args...)
	require.Equal(t, 0, res1.Code, "stderr: %s", res1.Stderr)

	dir.Add(t, "001_create_users.sql", "create table users (id bigint primary key);")

	res2 := runCLI(t, args...)
	assert.NotEqual(t, 0, res2.Code)
}

func TestCLI_Apply_MissingRequiredDSN(t *testing.T) {
	t.Parallel()
	dir := NewMigrationDir(t)

	dir.Add(t, "001.sql", "select 1;")
	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001.sql"}},
	})

	res := runCLI(t, "apply", "--manifest", manifest)
	assert.NotEqual(t, 0, res.Code)
}

func TestCLI_Plan_HasPending(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
	})

	res := runCLI(t, "plan", "--dsn", pg.ConnStr, "--manifest", manifest, "--table", histTable)
	assert.Equal(t, 2, res.Code)
	assert.False(t, TableExists(t, pg.DB, "public", "users"))
	assert.Empty(t, QueryHistory(t, pg.DB, histTable))
	assert.Contains(t, res.Stdout, "001_create_users.sql")
}

func TestCLI_Plan_NothingPending(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
	})
	args := []string{"--dsn", pg.ConnStr, "--manifest", manifest, "--table", histTable}

	require.Equal(t, 0, runCLI(t, append([]string{"apply"}, args...)...).Code)

	res := runCLI(t, append([]string{"plan"}, args...)...)
	assert.Equal(t, 0, res.Code)
	assert.Contains(t, res.Stdout, "nothing to apply")
}

func TestCLI_Status(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	dir.Add(t, "002_add_email.sql", "alter table users add column email text;")

	partialManifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
	})
	res := runCLI(t, "apply", "--dsn", pg.ConnStr, "--manifest", partialManifest, "--table", histTable)
	require.Equal(t, 0, res.Code, "apply stderr: %s", res.Stderr)

	fullManifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
		{Files: []string{"002_add_email.sql"}},
	})
	res = runCLI(t, "status", "--dsn", pg.ConnStr, "--manifest", fullManifest, "--table", histTable)
	assert.Equal(t, 0, res.Code, "status stderr: %s", res.Stderr)
	assert.Contains(t, res.Stdout, "PATH")
	assert.Contains(t, res.Stdout, "001_create_users.sql")
	assert.Contains(t, res.Stdout, "002_add_email.sql")
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
