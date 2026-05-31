package executor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashmap-kz/gopgmigrate/v2/internal/history"
	"github.com/hashmap-kz/gopgmigrate/v2/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKindLabel(t *testing.T) {
	tests := []struct {
		mode manifest.Mode
		want string
	}{
		{manifest.ModeDefault, "once"},
		{manifest.ModeNoTx, "no-tx"},
		{manifest.ModeRepeatable, "repeatable"},
		{manifest.ModeRepeatableNoTx, "repeatable-notx"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			e := manifest.Entry{Mode: tc.mode}
			assert.Equal(t, tc.want, kindLabel(e))
		})
	}
}

func entry(rev int64, mode manifest.Mode) manifest.Entry {
	return manifest.Entry{Revision: rev, Mode: mode}
}

func row(kind, checksum string) history.Row {
	return history.Row{Kind: kind, Checksum: checksum}
}

func mf(entries ...manifest.Entry) *manifest.Manifest {
	return &manifest.Manifest{Entries: entries}
}

func TestIntegrityCheck(t *testing.T) {
	t.Run("empty manifest and history", func(t *testing.T) {
		err := integrityCheck(mf(), map[int64]history.Row{})
		assert.NoError(t, err)
	})

	t.Run("history empty nothing to check", func(t *testing.T) {
		err := integrityCheck(mf(entry(1, manifest.ModeDefault)), map[int64]history.Row{})
		assert.NoError(t, err)
	})

	t.Run("matching revisions and kinds", func(t *testing.T) {
		err := integrityCheck(
			mf(entry(1, manifest.ModeDefault), entry(2, manifest.ModeRepeatable)),
			map[int64]history.Row{1: row("once", "abc"), 2: row("repeatable", "def")},
		)
		assert.NoError(t, err)
	})

	t.Run("revision in history but not in scan", func(t *testing.T) {
		err := integrityCheck(
			mf(entry(1, manifest.ModeDefault)),
			map[int64]history.Row{1: row("once", "abc"), 99: row("once", "xyz")},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "0000099")
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("mode changed from once to repeatable", func(t *testing.T) {
		err := integrityCheck(
			mf(entry(1, manifest.ModeRepeatable)),
			map[int64]history.Row{1: row("once", "abc")},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "0000001")
		assert.Contains(t, err.Error(), "once")
		assert.Contains(t, err.Error(), "repeatable")
	})

	t.Run("mode changed from repeatable to once", func(t *testing.T) {
		err := integrityCheck(
			mf(entry(1, manifest.ModeDefault)),
			map[int64]history.Row{1: row("repeatable", "abc")},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "0000001")
		assert.Contains(t, err.Error(), "repeatable")
		assert.Contains(t, err.Error(), "once")
	})

	t.Run("both missing and mode changed reported together", func(t *testing.T) {
		err := integrityCheck(
			mf(entry(1, manifest.ModeRepeatable)),
			map[int64]history.Row{
				1:  row("once", "abc"),
				99: row("once", "xyz"),
			},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "0000099")
		assert.Contains(t, err.Error(), "not found")
		assert.Contains(t, err.Error(), "0000001")
		assert.Contains(t, err.Error(), "mode")
	})

	t.Run("multiple missing revisions all reported", func(t *testing.T) {
		err := integrityCheck(
			mf(),
			map[int64]history.Row{1: row("once", "a"), 2: row("once", "b")},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "0000001")
		assert.Contains(t, err.Error(), "0000002")
	})
}

func TestChecksumGuard(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "migration.sql")
	require.NoError(t, os.WriteFile(path, []byte("select 1;"), 0o600))

	checksum, err := manifest.Checksum(path)
	require.NoError(t, err)

	t.Run("matching checksum passes", func(t *testing.T) {
		assert.NoError(t, checksumGuard(path, checksum))
	})

	t.Run("mismatched checksum is error", func(t *testing.T) {
		err := checksumGuard(path, "0000000000000000000000000000000000000000000000000000000000000000")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "checksum mismatch")
	})

	t.Run("missing file is error", func(t *testing.T) {
		err := checksumGuard(filepath.Join(dir, "nonexistent.sql"), checksum)
		require.Error(t, err)
	})
}
