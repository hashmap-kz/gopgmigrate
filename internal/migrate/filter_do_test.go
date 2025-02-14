package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopgmigrate/internal/history"
)

func TestCheckHistory(t *testing.T) {
	tests := []struct {
		name        string
		applied     []history.MigrateHistory
		files       []MigrationFile
		expectError bool
		expectedErr string
	}{
		{
			name: "All applied migrations exist locally",
			applied: []history.MigrateHistory{
				{MhName: "001-init.do.sql"},
				{MhName: "002-users.do.sql"},
			},
			files: []MigrationFile{
				{Base: "001-init.do.sql"},
				{Base: "002-users.do.sql"},
			},
			expectError: false,
		},
		{
			name: "Applied migration missing in local files",
			applied: []history.MigrateHistory{
				{MhName: "001-init.do.sql"},
				{MhName: "003-missing.sql"},
			},
			files: []MigrationFile{
				{Base: "001-init.do.sql"},
				{Base: "002-users.do.sql"},
			},
			expectError: true,
			expectedErr: "detected applied migration not resolved locally: 003-missing.sql",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := checkAppliedHistoryWithLocalFiles(test.applied, test.files)
			if test.expectError {
				assert.Error(t, err)
				assert.EqualError(t, err, test.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFound(t *testing.T) {
	tests := []struct {
		name      string
		searchKey string
		files     []MigrationFile
		wantFound bool
	}{
		{
			name:      "Migration exists",
			searchKey: "001-init.do.sql",
			files: []MigrationFile{
				{Base: "001-init.do.sql"},
				{Base: "002-users.do.sql"},
			},
			wantFound: true,
		},
		{
			name:      "Migration does not exist",
			searchKey: "003-missing.sql",
			files: []MigrationFile{
				{Base: "001-init.do.sql"},
				{Base: "002-users.do.sql"},
			},
			wantFound: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := appliedMigrationPresentLocally(test.searchKey, test.files)
			assert.Equal(t, test.wantFound, result)
		})
	}
}

func TestGetVersionedMigrationsToApply(t *testing.T) {
	mockFiles := []MigrationFile{
		{Base: "00100200300001-init.do.sql", Path: "/migrations/00100200300001-init.do.sql", data: []byte("init"), hash: "1"},
		{Base: "00100200300002--users.do.sql", Path: "/migrations/00100200300002--users.do.sql", data: []byte("users")},
	}

	mockHistory := []history.MigrateHistory{
		{MhName: "00100200300001-init.do.sql", MhHash: "1"},
	}

	toApply, err := getVersionedMigrationsToApply(mockHistory, mockFiles)
	assert.NoError(t, err)
	assert.Len(t, toApply, 1)
	assert.Equal(t, "00100200300002--users.do.sql", toApply[0].Base)

	// Test hash mismatch scenario
	mockHistory = append(mockHistory, history.MigrateHistory{MhName: "00100200300002--users.do.sql", MhHash: "wrong-hash"})
	_, err = getVersionedMigrationsToApply(mockHistory, mockFiles)
	assert.Error(t, err)
}

func TestFindHist(t *testing.T) {
	mockHistory := []history.MigrateHistory{
		{MhName: "00100200300001-init.do.sql", MhHash: "hash1"},
	}

	found := findHist("00100200300001-init.do.sql", mockHistory)
	assert.NotNil(t, found)
	assert.Equal(t, "hash1", found.MhHash)

	notFound := findHist("00100200300002--users.do.sql", mockHistory)
	assert.Nil(t, notFound)
}
