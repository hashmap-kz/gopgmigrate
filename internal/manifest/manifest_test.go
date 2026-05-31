package manifest

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

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

	mf, err := Scan(dir)
	require.NoError(t, err)
	assert.Equal(t, "schema_migrations", mf.Table)
}

func TestScan_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	mf, err := Scan(dir)
	require.NoError(t, err)
	assert.Empty(t, mf.Entries)
}

func TestScan_MissingDirectory(t *testing.T) {
	_, err := Scan("/nonexistent/path")
	require.Error(t, err)
}

func TestScan_SortedByRevision(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "0000003-c.up.sql")
	sqlFile(t, dir, "0000001-a.up.sql")
	sqlFile(t, dir, "0000002-b.up.sql")

	mf, err := Scan(dir)
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

	_, err := Scan(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate revision")
}

func TestScan_AllModes(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "0000001-versioned.up.sql")
	sqlFile(t, dir, "0000002-repeatable.r.sql")
	sqlFile(t, dir, "0000003-notx.notx.sql")
	sqlFile(t, dir, "0000004-repeatable-notx.rnotx.sql")

	mf, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, mf.Entries, 4)
	assert.Equal(t, ModeDefault, mf.Entries[0].Mode)
	assert.Equal(t, ModeRepeatable, mf.Entries[1].Mode)
	assert.Equal(t, ModeNoTx, mf.Entries[2].Mode)
	assert.Equal(t, ModeRepeatableNoTx, mf.Entries[3].Mode)
}

func TestScan_StrayFileIsError(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "0000001-init.up.sql")
	writeFile(t, filepath.Join(dir, "README.md"), "# docs")

	_, err := Scan(dir)
	require.Error(t, err)

	var stray *StrayFilesError
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

	_, err := Scan(dir)
	require.Error(t, err)

	var stray *StrayFilesError
	require.ErrorAs(t, err, &stray)
	assert.Len(t, stray.Files, 3)
}

func TestScan_ScansSubdirectories(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "0000001-init.up.sql")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0o755))
	writeFile(t, filepath.Join(dir, "subdir", "0000002-nested.up.sql"), "select 1;")

	mf, err := Scan(dir)
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

	_, err := Scan(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate revision")
}

func TestScan_StrayFileInSubdir(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "0000001-init.up.sql")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0o755))
	writeFile(t, filepath.Join(dir, "subdir", "notes.txt"), "")

	_, err := Scan(dir)
	require.Error(t, err)

	var stray *StrayFilesError
	require.ErrorAs(t, err, &stray)
	assert.Contains(t, stray.Files[0], "notes.txt")
}

func TestScan_PathsResolved(t *testing.T) {
	dir := t.TempDir()
	sqlFile(t, dir, "0000001-init.up.sql")

	mf, err := Scan(dir)
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

	mf, err := Scan(dir)
	require.NoError(t, err)
	assert.Equal(t, "0000001-create-users-table", mf.Entries[0].ID)
	assert.Equal(t, "0000002-refresh-stats", mf.Entries[1].ID)
	assert.Equal(t, "0000003-vacuum", mf.Entries[2].ID)
	assert.Equal(t, "0000004-rebuild-view", mf.Entries[3].ID)
}

func TestScan_StrayFilesErrorMessage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad.sql"), "")

	_, err := Scan(dir)
	require.Error(t, err)

	var stray *StrayFilesError
	require.True(t, errors.As(err, &stray))
	assert.Contains(t, err.Error(), "bad.sql")
	assert.Contains(t, err.Error(), "stray")
}

func TestChecksum_Deterministic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.sql")
	writeFile(t, path, "select 1;")

	sum1, err := Checksum(path)
	require.NoError(t, err)
	assert.NotEmpty(t, sum1)

	sum2, err := Checksum(path)
	require.NoError(t, err)
	assert.Equal(t, sum1, sum2)
}

func TestChecksum_ChangesWithContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.sql")

	writeFile(t, path, "select 1;")
	sum1, err := Checksum(path)
	require.NoError(t, err)

	writeFile(t, path, "select 2;")
	sum2, err := Checksum(path)
	require.NoError(t, err)

	assert.NotEqual(t, sum1, sum2)
}

func TestChecksum_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.sql")
	writeFile(t, path, "")

	sum, err := Checksum(path)
	require.NoError(t, err)
	assert.NotEmpty(t, sum)
}

func TestChecksum_MissingFile(t *testing.T) {
	_, err := Checksum("/nonexistent/file.sql")
	require.Error(t, err)
}

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.sql")
	writeFile(t, path, "select 42;")

	got, err := ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "select 42;", got)
}

func TestReadFile_MissingFile(t *testing.T) {
	_, err := ReadFile("/nonexistent/file.sql")
	require.Error(t, err)
}

// parsers

func TestParseFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantRev  int64
		wantStem string
		wantMode Mode
		wantOK   bool
	}{
		{
			name:     "up.sql",
			input:    "0000001-schemas.up.sql",
			wantRev:  1,
			wantStem: "0000001-schemas",
			wantMode: ModeDefault,
			wantOK:   true,
		},
		{
			name:     "r.sql",
			input:    "0000002-refresh-stats.r.sql",
			wantRev:  2,
			wantStem: "0000002-refresh-stats",
			wantMode: ModeRepeatable,
			wantOK:   true,
		},
		{
			name:     "notx.sql",
			input:    "0000003-vacuum.notx.sql",
			wantRev:  3,
			wantStem: "0000003-vacuum",
			wantMode: ModeNoTx,
			wantOK:   true,
		},
		{
			name:     "rnotx.sql",
			input:    "0000004-rebuild-view.rnotx.sql",
			wantRev:  4,
			wantStem: "0000004-rebuild-view",
			wantMode: ModeRepeatableNoTx,
			wantOK:   true,
		},
		{
			name:     "max revision",
			input:    "9999999-last.up.sql",
			wantRev:  9999999,
			wantStem: "9999999-last",
			wantMode: ModeDefault,
			wantOK:   true,
		},
		{
			name:     "multi-dash name",
			input:    "0000001-create-users-table.up.sql",
			wantRev:  1,
			wantStem: "0000001-create-users-table",
			wantMode: ModeDefault,
			wantOK:   true,
		},
		{
			name:     "dots in name",
			input:    "0000001-a.b.c.up.sql",
			wantRev:  1,
			wantStem: "0000001-a.b.c",
			wantMode: ModeDefault,
			wantOK:   true,
		},
		{
			name:   "empty string",
			input:  "",
			wantOK: false,
		},
		{
			name:   "no revision prefix",
			input:  "schemas.up.sql",
			wantOK: false,
		},
		{
			name:   "revision too short",
			input:  "000001-name.up.sql",
			wantOK: false,
		},
		{
			name:   "revision too long",
			input:  "00000001-name.up.sql",
			wantOK: false,
		},
		{
			name:   "non-digit revision",
			input:  "abcdefg-name.up.sql",
			wantOK: false,
		},
		{
			name:   "underscore separator",
			input:  "0000001_name.up.sql",
			wantOK: false,
		},
		{
			name:   "down migration",
			input:  "0000001-name.down.sql",
			wantOK: false,
		},
		{
			name:   "no kind extension",
			input:  "0000001-name.sql",
			wantOK: false,
		},
		{
			name:   "unknown extension",
			input:  "0000001-name.up.sql.bak",
			wantOK: false,
		},
		{
			name:   "plain sql file",
			input:  "plain.sql",
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseFilename(tc.input)
			require.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				assert.Equal(t, tc.wantRev, got.revision)
				assert.Equal(t, tc.wantStem, got.stem)
				assert.Equal(t, tc.wantMode, got.mode)
			}
		})
	}
}

func TestKindToMode(t *testing.T) {
	tests := []struct {
		kind string
		want Mode
	}{
		{"up", ModeDefault},
		{"r", ModeRepeatable},
		{"notx", ModeNoTx},
		{"rnotx", ModeRepeatableNoTx},
		{"", ModeDefault},
		{"unknown", ModeDefault},
	}
	for _, tc := range tests {
		t.Run(tc.kind, func(t *testing.T) {
			assert.Equal(t, tc.want, kindToMode(tc.kind))
		})
	}
}
