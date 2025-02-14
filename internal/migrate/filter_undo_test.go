package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopgmigrate/internal/history"
)

func TestGetVersionedMigrationsToUndo(t *testing.T) {
	files := []MigrationFile{
		{Base: "00001-init.undo.sql"},
		{Base: "00002-users.undo.sql"},
	}

	historyRecords := []history.MigrateHistory{
		{MhName: "00002-users.do.sql"},
		{MhName: "00001-init.do.sql"},
	}

	result, err := getVersionedMigrationsToUndo(files, historyRecords, 2)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "00002-users.undo.sql", result[0].Base)
	assert.Equal(t, "00001-init.undo.sql", result[1].Base)
}

func TestGetVersionedMigrationsToUndo_ExceedingRollbackCount(t *testing.T) {
	files := []MigrationFile{
		{Base: "00001-init.undo.sql"},
	}

	historyRecords := []history.MigrateHistory{
		{MhName: "00002-users.do.sql"},
		{MhName: "00001-init.do.sql"},
	}

	_, err := getVersionedMigrationsToUndo(files, historyRecords, 3)
	assert.Error(t, err)
	assert.Equal(t, "rollback-count is greater that the whole history", err.Error())
}

func TestFindCorrespondingUndoScript(t *testing.T) {
	undoFiles := []MigrationFile{
		{Base: "00001-init.undo.sql"},
		{Base: "00002-users.undo.sql"},
	}

	doScript := history.MigrateHistory{MhName: "00002-users.do.sql"}

	result, found, err := findCorrespondingUndoScript(undoFiles, doScript)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "00002-users.undo.sql", result.Base)
}

func TestFindCorrespondingUndoScript_NotFound(t *testing.T) {
	undoFiles := []MigrationFile{
		{Base: "00001-init.undo.sql"},
	}

	doScript := history.MigrateHistory{MhName: "00002-users.do.sql"}

	_, found, err := findCorrespondingUndoScript(undoFiles, doScript)
	assert.NoError(t, err)
	assert.False(t, found)
}

func TestFindCorrespondingUndoScript_ParseError(t *testing.T) {
	undoFiles := []MigrationFile{
		{Base: "invalid-undo.sql"},
	}

	doScript := history.MigrateHistory{MhName: "00002-users.do.sql"}

	_, _, err := findCorrespondingUndoScript(undoFiles, doScript)
	assert.Error(t, err)
}
