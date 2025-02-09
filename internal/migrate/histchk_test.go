package migrate

import "testing"

func TestCheckHistory(t *testing.T) {
	tests := []struct {
		name        string
		applied     map[string]bool
		files       *MigrationCtx
		expectError bool
		expectedErr string
	}{
		{
			name: "All applied migrations exist locally",
			applied: map[string]bool{
				"001-init.do.sql":  true,
				"002-users.do.sql": true,
			},
			files: &MigrationCtx{
				versioned: []migrationFile{
					{base: "001-init.do.sql"},
					{base: "002-users.do.sql"},
				},
				repeatable: []migrationFile{},
			},
			expectError: false,
		},
		{
			name: "Applied migration missing in local files",
			applied: map[string]bool{
				"001-init.do.sql": true,
				"003-missing.sql": true, // Missing file
			},
			files: &MigrationCtx{
				versioned: []migrationFile{
					{base: "001-init.do.sql"},
					{base: "002-users.do.sql"},
				},
				repeatable: []migrationFile{},
			},
			expectError: true,
			expectedErr: "detected applied migration not resolved locally: 003-missing.sql",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := checkHistory(test.applied, test.files)
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
		migrations  map[string]bool
		files       []migrationFile
		expectError bool
		expectedErr string
	}{
		{
			name: "All migrations exist locally",
			migrations: map[string]bool{
				"001-init.do.sql":  true,
				"002-users.do.sql": true,
			},
			files: []migrationFile{
				{base: "001-init.do.sql"},
				{base: "002-users.do.sql"},
			},
			expectError: false,
		},
		{
			name: "A migration is missing locally",
			migrations: map[string]bool{
				"001-init.do.sql": true,
				"003-missing.sql": true,
			},
			files: []migrationFile{
				{base: "001-init.do.sql"},
				{base: "002-users.do.sql"},
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
		files     []migrationFile
		wantFound bool
	}{
		{
			name:      "Migration exists",
			searchKey: "001-init.do.sql",
			files: []migrationFile{
				{base: "001-init.do.sql"},
				{base: "002-users.do.sql"},
			},
			wantFound: true,
		},
		{
			name:      "Migration does not exist",
			searchKey: "003-missing.sql",
			files: []migrationFile{
				{base: "001-init.do.sql"},
				{base: "002-users.do.sql"},
			},
			wantFound: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := found(test.searchKey, test.files)
			if result != test.wantFound {
				t.Errorf("found(%q) = %v, want %v", test.searchKey, result, test.wantFound)
			}
		})
	}
}
