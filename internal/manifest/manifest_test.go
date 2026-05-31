package manifest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashmap-kz/gopgmigrate/v2/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestLoad_DefaultTable(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "a.sql")
	path := writeManifest(t, dir, "migrations:\n  - id: v1\n    files: [a.sql]\n")

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
  - id: v1
    files: [a.sql]
`)
	mf, err := manifest.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "myschema.migrations", mf.Table)
}

func TestLoad_PathsResolvedRelativeToManifest(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "a.sql")
	path := writeManifest(t, dir, "migrations:\n  - id: v1\n    files: [a.sql]\n")

	mf, err := manifest.Load(path)
	require.NoError(t, err)
	require.Len(t, mf.Entries, 1)
	require.Len(t, mf.Entries[0].Files, 1)
	f := mf.Entries[0].Files[0]
	assert.Equal(t, "a.sql", f.Path)
	assert.Equal(t, filepath.Join(dir, "a.sql"), f.AbsPath)
}

func TestLoad_IDPropagated(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "a.sql")
	path := writeManifest(t, dir, "migrations:\n  - id: rel-1.0\n    files: [a.sql]\n")

	mf, err := manifest.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "rel-1.0", mf.Entries[0].ID)
}

func TestLoad_AllModes(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"a.sql", "b.sql", "c.sql", "d.sql"} {
		sqlFile(t, dir, f)
	}
	path := writeManifest(t, dir, `
migrations:
  - id: v1
    files: [a.sql]
  - id: v2
    files: [b.sql]
    mode: atomic
  - id: v3
    files: [c.sql]
    mode: no-tx
  - id: v4
    files: [d.sql]
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
  - id: v1
    files: [a.sql]
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
  - id: v1
    files: [a.sql, b.sql]
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

func TestLoad_MissingID(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "a.sql")
	path := writeManifest(t, dir, "migrations:\n  - files: [a.sql]\n")
	_, err := manifest.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id")
}

func TestLoad_InvalidIDChars(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "a.sql")
	path := writeManifest(t, dir, `
migrations:
  - id: "rel 1.0"
    files: [a.sql]
`)
	_, err := manifest.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid characters")
}

func TestLoad_DuplicateID(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "a.sql")
	sqlFile(t, dir, "b.sql")
	path := writeManifest(t, dir, `
migrations:
  - id: v1
    files: [a.sql]
  - id: v1
    files: [b.sql]
`)
	_, err := manifest.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "v1")
	assert.Contains(t, err.Error(), "not unique")
}

func TestLoad_EmptyFiles(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, "migrations:\n  - id: v1\n    files: []\n")
	_, err := manifest.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "files")
}

func TestLoad_UnknownMode(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "a.sql")
	path := writeManifest(t, dir, `
migrations:
  - id: v1
    files: [a.sql]
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
  - id: v1
    files: [a.sql, b.sql]
    mode: repeatable
`)
	mf, err := manifest.Load(path)
	require.NoError(t, err)
	assert.Len(t, mf.Entries[0].Files, 2)
}

func TestLoad_DuplicatePathsWithinEntry(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "a.sql")
	path := writeManifest(t, dir, `
migrations:
  - id: v1
    files: [a.sql, a.sql]
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
  - id: v1
    files: [a.sql]
  - id: v2
    files: [a.sql]
`)
	_, err := manifest.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestLoad_DuplicateBasenameWithinEntry(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "v1"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "v2"), 0o755))
	writeFile(t, filepath.Join(dir, "v1", "setup.sql"), "select 1;")
	writeFile(t, filepath.Join(dir, "v2", "setup.sql"), "select 2;")
	path := writeManifest(t, dir, `
migrations:
  - id: rel-1
    mode: atomic
    files: [v1/setup.sql, v2/setup.sql]
`)
	_, err := manifest.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "migration_id")
}

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
