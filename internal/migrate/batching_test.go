package migrate

import (
	"reflect"
	"testing"
)

func TestBatchResolving1(t *testing.T) {
	tests := []struct {
		name     string
		input    []MigrationFile
		expected []BatchEntries
	}{
		{
			name: "5-batches",

			input: []MigrationFile{
				{Base: "00000-audit-table.do.sql"},           // 1
				{Base: "00001-users-table.do.sql"},           // 1
				{Base: "00002-roles-table.do.sql"},           // 1
				{Base: "00003-privileges.do.sql"},            // 1
				{Base: "00004-users.do.sql"},                 // 1
				{Base: "00005-roles.do.sql"},                 // 1
				{Base: "00006-non-transactional.ntx.do.sql"}, // 2
				{Base: "00007-non-transactional.ntx.do.sql"}, // 2
				{Base: "00008-fn_get_users.r.sql"},           // 3
				{Base: "00009-fn_get_roles.r.sql"},           // 3
				{Base: "00010-alter-system.ntx.do.sql"},      // 4
				{Base: "00011-empty.do.sql"},                 // 5
				{Base: "00012-empty.do.sql"},                 // 5
			},
			expected: []BatchEntries{
				{
					Files: []MigrationFile{
						{Base: "00000-audit-table.do.sql"}, // 1
						{Base: "00001-users-table.do.sql"}, // 1
						{Base: "00002-roles-table.do.sql"}, // 1
						{Base: "00003-privileges.do.sql"},  // 1
						{Base: "00004-users.do.sql"},       // 1
						{Base: "00005-roles.do.sql"},       // 1
					},
					UseTX: true,
				},
				{
					Files: []MigrationFile{
						{Base: "00006-non-transactional.ntx.do.sql"}, // 2
						{Base: "00007-non-transactional.ntx.do.sql"}, // 2
					},
					UseTX: false,
				},
				{
					Files: []MigrationFile{
						{Base: "00008-fn_get_users.r.sql"}, // 3
						{Base: "00009-fn_get_roles.r.sql"}, // 3
					},
					UseTX: true,
				},
				{
					Files: []MigrationFile{
						{Base: "00010-alter-system.ntx.do.sql"}, // 4
					},
					UseTX: false,
				},
				{
					Files: []MigrationFile{
						{Base: "00011-empty.do.sql"}, // 5
						{Base: "00012-empty.do.sql"}, // 5
					},
					UseTX: true,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries, err := parseFilesIntoBatchEntries(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != len(test.expected) {
				t.Fatalf("test fail: %s", test.name)
			}
			for i := 0; i < len(entries); i++ {
				if !reflect.DeepEqual(test.expected[i], entries[i]) {
					t.Fatalf("test fail: %s", test.name)
				}
			}
		})
	}
}

func TestBatchResolving2(t *testing.T) {
	tests := []struct {
		name     string
		input    []MigrationFile
		expected []BatchEntries
	}{
		{
			name: "mix-1",

			input: []MigrationFile{
				{Base: "00000-audit-table.do.sql"},           // 1
				{Base: "00006-non-transactional.ntx.do.sql"}, // 2
			},
			expected: []BatchEntries{
				{
					Files: []MigrationFile{
						{Base: "00000-audit-table.do.sql"}, // 1
					},
					UseTX: true,
				},
				{
					Files: []MigrationFile{
						{Base: "00006-non-transactional.ntx.do.sql"}, // 2
					},
					UseTX: false,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries, err := parseFilesIntoBatchEntries(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != len(test.expected) {
				t.Fatalf("test fail: %s", test.name)
			}
			for i := 0; i < len(entries); i++ {
				if !reflect.DeepEqual(test.expected[i], entries[i]) {
					t.Fatalf("test fail: %s", test.name)
				}
			}
		})
	}
}

func TestBatchResolving3(t *testing.T) {
	tests := []struct {
		name     string
		input    []MigrationFile
		expected []BatchEntries
	}{
		{
			name: "tx-only",

			input: []MigrationFile{
				{Base: "00000-audit-table.do.sql"}, // 1
			},
			expected: []BatchEntries{
				{
					Files: []MigrationFile{
						{Base: "00000-audit-table.do.sql"}, // 1
					},
					UseTX: true,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries, err := parseFilesIntoBatchEntries(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != len(test.expected) {
				t.Fatalf("test fail: %s", test.name)
			}
			for i := 0; i < len(entries); i++ {
				if !reflect.DeepEqual(test.expected[i], entries[i]) {
					t.Fatalf("test fail: %s", test.name)
				}
			}
		})
	}
}

func TestBatchResolving4(t *testing.T) {
	tests := []struct {
		name     string
		input    []MigrationFile
		expected []BatchEntries
	}{
		{
			name: "notx-only",

			input: []MigrationFile{
				{Base: "00006-non-transactional.ntx.do.sql"}, // 1
			},
			expected: []BatchEntries{
				{
					Files: []MigrationFile{
						{Base: "00006-non-transactional.ntx.do.sql"}, // 1
					},
					UseTX: false,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries, err := parseFilesIntoBatchEntries(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != len(test.expected) {
				t.Fatalf("test fail: %s", test.name)
			}
			for i := 0; i < len(entries); i++ {
				if !reflect.DeepEqual(test.expected[i], entries[i]) {
					t.Fatalf("test fail: %s", test.name)
				}
			}
		})
	}
}
