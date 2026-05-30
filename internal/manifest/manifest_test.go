package manifest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashmap-kz/gopgmigrate/v2/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helpers

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func writeManifest(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "manifest.yaml")
	writeFile(t, path, content)
	return path
}

func sqlFile(t *testing.T, dir, name string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, name), "select 1;")
}

// Load

func TestLoad_DefaultTable(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "a.sql")
	path := writeManifest(t, dir, "migrations:\n  - files: [a.sql]\n")

	mf, err := manifest.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "schema_migrations", mf.Table)
}

func TestLoad_CustomTable(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "a.sql")
	path := writeManifest(t, dir, `
manifest:
  table: myschema.migrations
migrations:
  - files: [a.sql]
`)
	mf, err := manifest.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "myschema.migrations", mf.Table)
}

func TestLoad_PathsResolvedRelativeToManifest(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "a.sql")
	path := writeManifest(t, dir, "migrations:\n  - files: [a.sql]\n")

	mf, err := manifest.Load(path)
	require.NoError(t, err)
	require.Len(t, mf.Entries, 1)
	require.Len(t, mf.Entries[0].Files, 1)
	assert.Equal(t, filepath.Join(dir, "a.sql"), mf.Entries[0].Files[0])
}

func TestLoad_AllModes(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"a.sql", "b.sql", "c.sql", "d.sql"} {
		sqlFile(t, dir, f)
	}
	path := writeManifest(t, dir, `
migrations:
  - files: [a.sql]
  - files: [b.sql]
    mode: atomic
  - files: [c.sql]
    mode: no-tx
  - files: [d.sql]
    mode: repeatable
`)
	mf, err := manifest.Load(path)
	require.NoError(t, err)
	require.Len(t, mf.Entries, 4)
	assert.Equal(t, manifest.ModeDefault, mf.Entries[0].Mode)
	assert.Equal(t, manifest.ModeAtomic, mf.Entries[1].Mode)
	assert.Equal(t, manifest.ModeNoTx, mf.Entries[2].Mode)
	assert.Equal(t, manifest.ModeRepeatable, mf.Entries[3].Mode)
}

func TestLoad_Description(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "a.sql")
	path := writeManifest(t, dir, `
migrations:
  - files: [a.sql]
    description: "release-1.0"
`)
	mf, err := manifest.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "release-1.0", mf.Entries[0].Description)
}

func TestLoad_MultipleFilesInAtomicEntry(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "a.sql")
	sqlFile(t, dir, "b.sql")
	path := writeManifest(t, dir, `
migrations:
  - files: [a.sql, b.sql]
    mode: atomic
`)
	mf, err := manifest.Load(path)
	require.NoError(t, err)
	assert.Len(t, mf.Entries[0].Files, 2)
}

func TestLoad_EmptyMigrationsList(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, "migrations: []\n")

	mf, err := manifest.Load(path)
	require.NoError(t, err)
	assert.Empty(t, mf.Entries)
}

// Load: error cases

func TestLoad_MissingManifestFile(t *testing.T) {
	_, err := manifest.Load("/nonexistent/path/manifest.yaml")
	require.Error(t, err)
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, "{")
	_, err := manifest.Load(path)
	require.Error(t, err)
}

func TestLoad_EmptyFiles(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, "migrations:\n  - files: []\n")
	_, err := manifest.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "files")
}

func TestLoad_UnknownMode(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "a.sql")
	path := writeManifest(t, dir, `
migrations:
  - files: [a.sql]
    mode: rollback
`)
	_, err := manifest.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollback")
}

func TestLoad_RepeatableMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "a.sql")
	sqlFile(t, dir, "b.sql")
	path := writeManifest(t, dir, `
migrations:
  - files: [a.sql, b.sql]
    mode: repeatable
`)
	_, err := manifest.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repeatable")
}

func TestLoad_DuplicatePathsWithinEntry(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "a.sql")
	path := writeManifest(t, dir, `
migrations:
  - files: [a.sql, a.sql]
    mode: atomic
`)
	_, err := manifest.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestLoad_DuplicatePathsAcrossEntries(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "a.sql")
	path := writeManifest(t, dir, `
migrations:
  - files: [a.sql]
  - files: [a.sql]
`)
	_, err := manifest.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

// Checksum

func TestChecksum_Deterministic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.sql")
	writeFile(t, path, "select 1;")

	sum1, err := manifest.Checksum(path)
	require.NoError(t, err)
	assert.NotEmpty(t, sum1)

	sum2, err := manifest.Checksum(path)
	require.NoError(t, err)
	assert.Equal(t, sum1, sum2)
}

func TestChecksum_ChangesWithContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.sql")

	writeFile(t, path, "select 1;")
	sum1, err := manifest.Checksum(path)
	require.NoError(t, err)

	writeFile(t, path, "select 2;")
	sum2, err := manifest.Checksum(path)
	require.NoError(t, err)

	assert.NotEqual(t, sum1, sum2)
}

func TestChecksum_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.sql")
	writeFile(t, path, "")

	sum, err := manifest.Checksum(path)
	require.NoError(t, err)
	assert.NotEmpty(t, sum)
}

func TestChecksum_MissingFile(t *testing.T) {
	_, err := manifest.Checksum("/nonexistent/file.sql")
	require.Error(t, err)
}

// ReadFile

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.sql")
	writeFile(t, path, "select 42;")

	got, err := manifest.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "select 42;", got)
}

func TestReadFile_MissingFile(t *testing.T) {
	_, err := manifest.ReadFile("/nonexistent/file.sql")
	require.Error(t, err)
}
