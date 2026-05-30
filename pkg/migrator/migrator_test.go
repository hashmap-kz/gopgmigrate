package migrator_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashmap-kz/gopgmigrate/v2/pkg/migrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helpers

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func writeManifest(t *testing.T, dir string, entries []string) string {
	t.Helper()
	var yaml string
	yaml += "migrations:\n"
	for _, f := range entries {
		yaml += "  - files: [" + f + "]\n"
	}
	path := filepath.Join(dir, "manifest.yaml")
	writeFile(t, path, yaml)
	return path
}

// NewValidateOnly

func TestNewValidateOnly_EmptyManifestPath(t *testing.T) {
	_, err := migrator.NewValidateOnly(migrator.Config{})
	require.Error(t, err)
}

// Validate

func TestValidate_OK(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "001.sql"), "create table t (id int);")
	manifest := writeManifest(t, dir, []string{"001.sql"})

	m, err := migrator.NewValidateOnly(migrator.Config{ManifestPath: manifest})
	require.NoError(t, err)
	require.NoError(t, m.Validate())
}

func TestValidate_MissingFile(t *testing.T) {
	dir := t.TempDir()
	// manifest references a file that was never written
	manifest := writeManifest(t, dir, []string{"nonexistent.sql"})

	m, err := migrator.NewValidateOnly(migrator.Config{ManifestPath: manifest})
	require.NoError(t, err)
	err = m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent.sql")
}

func TestValidate_MultipleFiles_AllMustExist(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "001.sql"), "select 1;")
	// 002.sql intentionally missing

	var yaml string
	yaml += "migrations:\n"
	yaml += "  - files: [001.sql, 002.sql]\n"
	yaml += "    mode: atomic\n"
	manifestPath := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifestPath, yaml)

	m, err := migrator.NewValidateOnly(migrator.Config{ManifestPath: manifestPath})
	require.NoError(t, err)
	err = m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "002.sql")
}
