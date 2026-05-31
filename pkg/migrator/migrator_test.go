package migrator_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashmap-kz/gopgmigrate/v2/pkg/migrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func TestNewWithDSN_EmptyDSN(t *testing.T) {
	_, err := migrator.NewWithDSN("", migrator.Config{Dir: "migrations"})
	require.Error(t, err)
}

func TestNewWithDB_NilDB(t *testing.T) {
	_, err := migrator.NewWithDB(nil, migrator.Config{Dir: "migrations"})
	require.Error(t, err)
}

func TestNewValidateOnly_EmptyDir(t *testing.T) {
	_, err := migrator.NewValidateOnly(migrator.Config{})
	require.Error(t, err)
}

func TestValidate_OK(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "0000001-init.up.sql"), "create table t (id int);")

	m, err := migrator.NewValidateOnly(migrator.Config{Dir: dir})
	require.NoError(t, err)
	require.NoError(t, m.Validate())
}

func TestValidate_MissingDirectory(t *testing.T) {
	m, err := migrator.NewValidateOnly(migrator.Config{Dir: "/nonexistent/path"})
	require.NoError(t, err)
	err = m.Validate()
	require.Error(t, err)
}

func TestValidate_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	m, err := migrator.NewValidateOnly(migrator.Config{Dir: dir})
	require.NoError(t, err)
	assert.NoError(t, m.Validate())
}
