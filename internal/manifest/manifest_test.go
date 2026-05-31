package manifest_test

import (
	"errors"
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

func sqlFile(t *testing.T, dir, name string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, name), "select 1;")
}

func TestScan_DefaultTable(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "0000001-init.up.sql")

	mf, err := manifest.Scan(dir)
	require.NoError(t, err)
	assert.Equal(t, "schema_migrations", mf.Table)
}

func TestScan_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	mf, err := manifest.Scan(dir)
	require.NoError(t, err)
	assert.Empty(t, mf.Entries)
}

func TestScan_MissingDirectory(t *testing.T) {
	_, err := manifest.Scan("/nonexistent/path")
	require.Error(t, err)
}

func TestScan_SortedByRevision(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "0000003-c.up.sql")
	sqlFile(t, dir, "0000001-a.up.sql")
	sqlFile(t, dir, "0000002-b.up.sql")

	mf, err := manifest.Scan(dir)
	require.NoError(t, err)
	require.Len(t, mf.Entries, 3)
	assert.Equal(t, "0000001-a", mf.Entries[0].ID)
	assert.Equal(t, "0000002-b", mf.Entries[1].ID)
	assert.Equal(t, "0000003-c", mf.Entries[2].ID)
}

func TestScan_DuplicateRevision(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "0000001-a.up.sql")
	sqlFile(t, dir, "0000001-b.notx.sql")

	_, err := manifest.Scan(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate revision")
}

func TestScan_AllModes(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "0000001-versioned.up.sql")
	sqlFile(t, dir, "0000002-repeatable.r.sql")
	sqlFile(t, dir, "0000003-notx.notx.sql")
	sqlFile(t, dir, "0000004-repeatable-notx.rnotx.sql")

	mf, err := manifest.Scan(dir)
	require.NoError(t, err)
	require.Len(t, mf.Entries, 4)
	assert.Equal(t, manifest.ModeDefault, mf.Entries[0].Mode)
	assert.Equal(t, manifest.ModeRepeatable, mf.Entries[1].Mode)
	assert.Equal(t, manifest.ModeNoTx, mf.Entries[2].Mode)
	assert.Equal(t, manifest.ModeRepeatableNoTx, mf.Entries[3].Mode)
}

func TestScan_StrayFileIsError(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "0000001-init.up.sql")
	writeFile(t, filepath.Join(dir, "README.md"), "# docs")

	_, err := manifest.Scan(dir)
	require.Error(t, err)

	var stray *manifest.StrayFilesError
	require.ErrorAs(t, err, &stray)
	require.Len(t, stray.Files, 1)
	assert.Contains(t, stray.Files[0], "README.md")
}

func TestScan_AllStrayFilesCollected(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "0000001-init.up.sql")
	writeFile(t, filepath.Join(dir, "README.md"), "")
	writeFile(t, filepath.Join(dir, "plain.sql"), "")
	writeFile(t, filepath.Join(dir, "0000002-rollback.down.sql"), "")

	_, err := manifest.Scan(dir)
	require.Error(t, err)

	var stray *manifest.StrayFilesError
	require.ErrorAs(t, err, &stray)
	assert.Len(t, stray.Files, 3)
}

func TestScan_ScansSubdirectories(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "0000001-init.up.sql")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0o755))
	writeFile(t, filepath.Join(dir, "subdir", "0000002-nested.up.sql"), "select 1;")

	mf, err := manifest.Scan(dir)
	require.NoError(t, err)
	require.Len(t, mf.Entries, 2)
	assert.Equal(t, "0000001-init.up.sql", mf.Entries[0].Files[0].Path)
	assert.Equal(t, "subdir/0000002-nested.up.sql", mf.Entries[1].Files[0].Path)
}

func TestScan_DuplicateRevisionAcrossSubdirs(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "a"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "b"), 0o755))
	writeFile(t, filepath.Join(dir, "a", "0000001-x.up.sql"), "select 1;")
	writeFile(t, filepath.Join(dir, "b", "0000001-y.up.sql"), "select 1;")

	_, err := manifest.Scan(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate revision")
}

func TestScan_StrayFileInSubdir(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "0000001-init.up.sql")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0o755))
	writeFile(t, filepath.Join(dir, "subdir", "notes.txt"), "")

	_, err := manifest.Scan(dir)
	require.Error(t, err)

	var stray *manifest.StrayFilesError
	require.ErrorAs(t, err, &stray)
	assert.Contains(t, stray.Files[0], "notes.txt")
}

func TestScan_PathsResolved(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "0000001-init.up.sql")

	mf, err := manifest.Scan(dir)
	require.NoError(t, err)
	require.Len(t, mf.Entries, 1)
	f := mf.Entries[0].Files[0]
	assert.Equal(t, "0000001-init.up.sql", f.Path)
	assert.Equal(t, filepath.Join(dir, "0000001-init.up.sql"), f.AbsPath)
}

func TestScan_IDIsStem(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "0000001-create-users-table.up.sql")
	sqlFile(t, dir, "0000002-refresh-stats.r.sql")
	sqlFile(t, dir, "0000003-vacuum.notx.sql")
	sqlFile(t, dir, "0000004-rebuild-view.rnotx.sql")

	mf, err := manifest.Scan(dir)
	require.NoError(t, err)
	assert.Equal(t, "0000001-create-users-table", mf.Entries[0].ID)
	assert.Equal(t, "0000002-refresh-stats", mf.Entries[1].ID)
	assert.Equal(t, "0000003-vacuum", mf.Entries[2].ID)
	assert.Equal(t, "0000004-rebuild-view", mf.Entries[3].ID)
}

func TestScan_StrayFilesErrorMessage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad.sql"), "")

	_, err := manifest.Scan(dir)
	require.Error(t, err)

	var stray *manifest.StrayFilesError
	require.True(t, errors.As(err, &stray))
	assert.Contains(t, err.Error(), "bad.sql")
	assert.Contains(t, err.Error(), "stray")
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
