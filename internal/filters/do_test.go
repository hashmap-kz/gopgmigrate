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
				{Name: "001-init.up.sql"},
				{Name: "002-users.up.sql"},
			},
			files: []naming.MigrationFile{
				{Base: "001-init.up.sql"},
				{Base: "002-users.up.sql"},
			},
			expectError: false,
		},
		{
			name: "Applied migration missing in local files",
			applied: []history.MigrateHistory{
				{Name: "001-init.up.sql"},
				{Name: "003-missing.sql"},
			},
			files: []naming.MigrationFile{
				{Base: "001-init.up.sql"},
				{Base: "002-users.up.sql"},
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
			searchKey: "001-init.up.sql",
			files: []naming.MigrationFile{
				{Base: "001-init.up.sql"},
				{Base: "002-users.up.sql"},
			},
			wantFound: true,
		},
		{
			name:      "Migration does not exist",
			searchKey: "003-missing.sql",
			files: []naming.MigrationFile{
				{Base: "001-init.up.sql"},
				{Base: "002-users.up.sql"},
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
		{Base: "0000001-init.up.sql", Path: "/migrations/0000001-init.up.sql", Data: []byte("init"), Hash: "1"},
		{Base: "0000002-users.up.sql", Path: "/migrations/0000002-users.up.sql", Data: []byte("users")},
	}

	//nolint:prealloc
	mockHistory := []history.MigrateHistory{
		{Name: "0000001-init.up.sql", Hash: "1"},
	}

	toApply, err := getVersionedMigrationsToApply(mockHistory, mockFiles)
	assert.NoError(t, err)
	assert.Len(t, toApply, 1)
	assert.Equal(t, "0000002-users.up.sql", toApply[0].Base)

	// Test hash mismatch scenario
	mockHistory = append(mockHistory, history.MigrateHistory{Name: "0000002-users.up.sql", Hash: "wrong-hash"})
	_, err = getVersionedMigrationsToApply(mockHistory, mockFiles)
	assert.Error(t, err)
}

func TestFindHist(t *testing.T) {
	mockHistory := []history.MigrateHistory{
		{Name: "00001-init.up.sql", Hash: "hash1"},
	}

	found := findHist("00001-init.up.sql", mockHistory)
	assert.NotNil(t, found)
	assert.Equal(t, "hash1", found.Hash)

	notFound := findHist("00002-users.up.sql", mockHistory)
	assert.Nil(t, notFound)
}
