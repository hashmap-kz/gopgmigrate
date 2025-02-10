package migrate

import (
	"testing"
)

func TestCheckHistory(t *testing.T) {
	tests := []struct {
		name        string
		applied     AppliedHistory
		files       []MigrationFile
		expectError bool
		expectedErr string
	}{
		{
			name: "All applied migrations exist locally",
			applied: AppliedHistory{
				"001-init.do.sql":  AppliedHistoryItem{},
				"002-users.do.sql": AppliedHistoryItem{},
			},
			files: []MigrationFile{
				{Base: "001-init.do.sql"},
				{Base: "002-users.do.sql"},
			},
			expectError: false,
		},
		{
			name: "Applied migration missing in local files",
			applied: AppliedHistory{
				"001-init.do.sql": AppliedHistoryItem{},
				"003-missing.sql": AppliedHistoryItem{}, // Missing file
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
				if err == nil {
					t.Errorf("Expected error but got nil")
				} else if err.Error() != test.expectedErr {
					t.Errorf("Expected error: %q, got: %q", test.expectedErr, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
			}
		})
	}
}

func TestCheckHistoryTableIsSyncedWithLocalFiles(t *testing.T) {
	tests := []struct {
		name        string
		migrations  AppliedHistory
		files       []MigrationFile
		expectError bool
		expectedErr string
	}{
		{
			name: "All migrations exist locally",
			migrations: AppliedHistory{
				"001-init.do.sql":  AppliedHistoryItem{},
				"002-users.do.sql": AppliedHistoryItem{},
			},
			files: []MigrationFile{
				{Base: "001-init.do.sql"},
				{Base: "002-users.do.sql"},
			},
			expectError: false,
		},
		{
			name: "A migration is missing locally",
			migrations: AppliedHistory{
				"001-init.do.sql": AppliedHistoryItem{},
				"003-missing.sql": AppliedHistoryItem{},
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
			err := checkHistoryTableIsSyncedWithLocalFiles(test.migrations, test.files)
			if test.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				} else if err.Error() != test.expectedErr {
					t.Errorf("Expected error: %q, got: %q", test.expectedErr, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
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
			if result != test.wantFound {
				t.Errorf("found(%q) = %v, want %v", test.searchKey, result, test.wantFound)
			}
		})
	}
}

// Test getVersionedMigrationsToApply function
func TestGetVersionedMigrationsToApply(t *testing.T) {
	mockFiles := []MigrationFile{
		{Base: "00001-init.do.sql", Path: "/migrations/00001-init.do.sql", data: []byte("init"), hash: "1"},
		{Base: "00002-users.do.sql", Path: "/migrations/00002-users.do.sql", data: []byte("users")},
	}

	mockHistory := AppliedHistory{
		"00001-init.do.sql": {MhName: "00001-init.do.sql", MhHash: "1"},
	}

	toApply, err := getVersionedMigrationsToApply(mockHistory, mockFiles)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(toApply) != 1 || toApply[0].Base != "00002-users.do.sql" {
		t.Errorf("Expected only 00002-users.do.sql to apply, got: %v", toApply)
	}

	// Test hash mismatch scenario
	mockHistory["00002-users.do.sql"] = AppliedHistoryItem{MhName: "00002-users.do.sql", MhHash: "wrong-hash"}
	_, err = getVersionedMigrationsToApply(mockHistory, mockFiles)
	if err == nil {
		t.Errorf("Expected hash mismatch error but got nil")
	}
}

// Test findHist function
func TestFindHist(t *testing.T) {
	mockHistory := AppliedHistory{
		"00001-init.do.sql": {MhName: "00001-init.do.sql", MhHash: "hash1"},
	}

	found := findHist("00001-init.do.sql", mockHistory)
	if found == nil || found.MhHash != "hash1" {
		t.Errorf("Expected to find migration 00001-init.do.sql, but got nil or incorrect data")
	}

	notFound := findHist("00002-users.do.sql", mockHistory)
	if notFound != nil {
		t.Errorf("Expected nil for non-existent migration, but got: %v", notFound)
	}
}
