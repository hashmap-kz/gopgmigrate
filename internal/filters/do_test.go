package filters

import (
	"testing"

	"gopgmigrate/internal/history"

	"gopgmigrate/internal/naming"

	"github.com/stretchr/testify/assert"
)

func TestCheckHistory(t *testing.T) {
	tests := []struct {
		name        string
		applied     []history.MigrateHistory
		files       []naming.MigrationFile
		expectError bool
		expectedErr string
	}{
		{
			name: "All applied migrations exist locally",
			applied: []history.MigrateHistory{
				{Name: "001-init.do.sql"},
				{Name: "002-users.do.sql"},
			},
			files: []naming.MigrationFile{
				{Base: "001-init.do.sql"},
				{Base: "002-users.do.sql"},
			},
			expectError: false,
		},
		{
			name: "Applied migration missing in local files",
			applied: []history.MigrateHistory{
				{Name: "001-init.do.sql"},
				{Name: "003-missing.sql"},
			},
			files: []naming.MigrationFile{
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
		files     []naming.MigrationFile
		wantFound bool
	}{
		{
			name:      "Migration exists",
			searchKey: "001-init.do.sql",
			files: []naming.MigrationFile{
				{Base: "001-init.do.sql"},
				{Base: "002-users.do.sql"},
			},
			wantFound: true,
		},
		{
			name:      "Migration does not exist",
			searchKey: "003-missing.sql",
			files: []naming.MigrationFile{
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
	mockFiles := []naming.MigrationFile{
		{Base: "00001-init.do.sql", Path: "/migrations/00001-init.do.sql", Data: []byte("init"), Hash: "1"},
		{Base: "00002-users.do.sql", Path: "/migrations/00002-users.do.sql", Data: []byte("users")},
	}

	mockHistory := []history.MigrateHistory{
		{Name: "00001-init.do.sql", Hash: "1"},
	}

	toApply, err := getVersionedMigrationsToApply(mockHistory, mockFiles)
	assert.NoError(t, err)
	assert.Len(t, toApply, 1)
	assert.Equal(t, "00002-users.do.sql", toApply[0].Base)

	// Test hash mismatch scenario
	mockHistory = append(mockHistory, history.MigrateHistory{Name: "00002-users.do.sql", Hash: "wrong-hash"})
	_, err = getVersionedMigrationsToApply(mockHistory, mockFiles)
	assert.Error(t, err)
}

func TestFindHist(t *testing.T) {
	mockHistory := []history.MigrateHistory{
		{Name: "00001-init.do.sql", Hash: "hash1"},
	}

	found := findHist("00001-init.do.sql", mockHistory)
	assert.NotNil(t, found)
	assert.Equal(t, "hash1", found.Hash)

	notFound := findHist("00002-users.do.sql", mockHistory)
	assert.Nil(t, notFound)
}
