package filters

import (
	"testing"

	"gopgmigrate/internal/history"

	"gopgmigrate/internal/naming"

	"github.com/stretchr/testify/assert"
)

func TestGetVersionedMigrationsToUndo(t *testing.T) {
	files := []naming.MigrationFile{
		{Base: "0000001-init.down.sql"},
		{Base: "0000002-users.down.sql"},
	}

	historyRecords := []history.MigrateHistory{
		{Name: "0000002-users.up.sql"},
		{Name: "0000001-init.up.sql"},
	}

	result, err := getVersionedMigrationsToUndo(files, historyRecords, 2)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "0000002-users.down.sql", result[0].Base)
	assert.Equal(t, "0000001-init.down.sql", result[1].Base)
}

func TestGetVersionedMigrationsToUndo_ExceedingRollbackCount(t *testing.T) {
	files := []naming.MigrationFile{
		{Base: "0000001-init.down.sql"},
	}

	historyRecords := []history.MigrateHistory{
		{Name: "0000002-users.up.sql"},
		{Name: "0000001-init.up.sql"},
	}

	_, err := getVersionedMigrationsToUndo(files, historyRecords, 3)
	assert.Error(t, err)
	assert.Equal(t, "rollback-count is greater that the whole history", err.Error())
}

func TestFindCorrespondingUndoScript(t *testing.T) {
	undoFiles := []naming.MigrationFile{
		{Base: "0000001-init.down.sql"},
		{Base: "0000002-users.down.sql"},
	}

	doScript := history.MigrateHistory{Name: "0000002-users.up.sql"}

	result, found, err := findCorrespondingUndoScript(undoFiles, doScript)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "0000002-users.down.sql", result.Base)
}

func TestFindCorrespondingUndoScript_NotFound(t *testing.T) {
	undoFiles := []naming.MigrationFile{
		{Base: "0000001-init.down.sql"},
	}

	doScript := history.MigrateHistory{Name: "0000002-users.up.sql"}

	_, found, err := findCorrespondingUndoScript(undoFiles, doScript)
	assert.NoError(t, err)
	assert.False(t, found)
}

func TestFindCorrespondingUndoScript_ParseError(t *testing.T) {
	undoFiles := []naming.MigrationFile{
		{Base: "invalid-undo.sql"},
	}

	doScript := history.MigrateHistory{Name: "0000002-users.up.sql"}

	_, _, err := findCorrespondingUndoScript(undoFiles, doScript)
	assert.Error(t, err)
}
