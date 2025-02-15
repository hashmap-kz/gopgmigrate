package modes

import (
	"reflect"
	"testing"

	"gopgmigrate/internal/vers"
)

func TestBatchResolving1(t *testing.T) {
	tests := []struct {
		name     string
		input    []vers.MigrationFile
		expected []GroupEntry
	}{
		{
			name: "5-batches",

			input: []vers.MigrationFile{
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
			expected: []GroupEntry{
				{
					Files: []vers.MigrationFile{
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
					Files: []vers.MigrationFile{
						{Base: "00006-non-transactional.ntx.do.sql"}, // 2
						{Base: "00007-non-transactional.ntx.do.sql"}, // 2
					},
					UseTX: false,
				},
				{
					Files: []vers.MigrationFile{
						{Base: "00008-fn_get_users.r.sql"}, // 3
						{Base: "00009-fn_get_roles.r.sql"}, // 3
					},
					UseTX: true,
				},
				{
					Files: []vers.MigrationFile{
						{Base: "00010-alter-system.ntx.do.sql"}, // 4
					},
					UseTX: false,
				},
				{
					Files: []vers.MigrationFile{
						{Base: "00011-empty.do.sql"}, // 5
						{Base: "00012-empty.do.sql"}, // 5
					},
					UseTX: true,
				},
			},
		},
	}

	checkMixedMode(t, tests)
}

func TestBatchResolving2(t *testing.T) {
	tests := []struct {
		name     string
		input    []vers.MigrationFile
		expected []GroupEntry
	}{
		{
			name: "mix-1",

			input: []vers.MigrationFile{
				{Base: "00000-audit-table.do.sql"},           // 1
				{Base: "00006-non-transactional.ntx.do.sql"}, // 2
			},
			expected: []GroupEntry{
				{
					Files: []vers.MigrationFile{
						{Base: "00000-audit-table.do.sql"}, // 1
					},
					UseTX: true,
				},
				{
					Files: []vers.MigrationFile{
						{Base: "00006-non-transactional.ntx.do.sql"}, // 2
					},
					UseTX: false,
				},
			},
		},
	}

	checkMixedMode(t, tests)
}

func TestBatchResolving3(t *testing.T) {
	tests := []struct {
		name     string
		input    []vers.MigrationFile
		expected []GroupEntry
	}{
		{
			name: "tx-only",

			input: []vers.MigrationFile{
				{Base: "00000-audit-table.do.sql"}, // 1
			},
			expected: []GroupEntry{
				{
					Files: []vers.MigrationFile{
						{Base: "00000-audit-table.do.sql"}, // 1
					},
					UseTX: true,
				},
			},
		},
	}

	checkMixedMode(t, tests)
}

func TestBatchResolving4(t *testing.T) {
	tests := []struct {
		name     string
		input    []vers.MigrationFile
		expected []GroupEntry
	}{
		{
			name: "notx-only",

			input: []vers.MigrationFile{
				{Base: "00006-non-transactional.ntx.do.sql"}, // 1
			},
			expected: []GroupEntry{
				{
					Files: []vers.MigrationFile{
						{Base: "00006-non-transactional.ntx.do.sql"}, // 1
					},
					UseTX: false,
				},
			},
		},
	}

	checkMixedMode(t, tests)
}

func TestBatchResolving5(t *testing.T) {
	tests := []struct {
		name     string
		input    []vers.MigrationFile
		expected []GroupEntry
	}{
		{
			name: "empty-input",

			input:    []vers.MigrationFile{},
			expected: nil,
		},
	}

	checkMixedMode(t, tests)
}

func TestBatchResolving6(t *testing.T) {
	tests := []struct {
		name        string
		input       []vers.MigrationFile
		expected    GroupEntry
		expectError bool
	}{
		{
			name: "group-mode-1",

			input: []vers.MigrationFile{
				{Base: "00006-non-transactional.ntx.do.sql"}, // 1
				{Base: "00007-non-transactional.ntx.do.sql"}, // 1
				{Base: "00008-non-transactional.ntx.do.sql"}, // 1
			},
			expected: GroupEntry{
				Files: []vers.MigrationFile{
					{Base: "00006-non-transactional.ntx.do.sql"}, // 1
					{Base: "00007-non-transactional.ntx.do.sql"}, // 1
					{Base: "00008-non-transactional.ntx.do.sql"}, // 1
				},
				UseTX: false,
			},
		},
	}

	checkGroupMode(t, tests)
}

func TestBatchResolving7(t *testing.T) {
	tests := []struct {
		name        string
		input       []vers.MigrationFile
		expected    GroupEntry
		expectError bool
	}{
		{
			name: "group-mode-2",

			input: []vers.MigrationFile{
				{Base: "00006-transactional.do.sql"}, // 1
				{Base: "00007-transactional.do.sql"}, // 1
				{Base: "00008-transactional.do.sql"}, // 1
			},
			expected: GroupEntry{
				Files: []vers.MigrationFile{
					{Base: "00006-transactional.do.sql"}, // 1
					{Base: "00007-transactional.do.sql"}, // 1
					{Base: "00008-transactional.do.sql"}, // 1
				},
				UseTX: true,
			},
		},
	}

	checkGroupMode(t, tests)
}

func TestBatchResolving8(t *testing.T) {
	tests := []struct {
		name        string
		input       []vers.MigrationFile
		expected    GroupEntry
		expectError bool
	}{
		{
			name: "group-mode-3",

			input: []vers.MigrationFile{
				{Base: "00006-transactional.do.sql"},         // 1
				{Base: "00007-non-transactional.ntx.do.sql"}, // should fail
			},
			expected:    GroupEntry{},
			expectError: true,
		},
	}

	checkGroupMode(t, tests)
}

func checkGroupMode(t *testing.T, tests []struct {
	name        string
	input       []vers.MigrationFile
	expected    GroupEntry
	expectError bool
},
) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g, err := ParseFilesGroupMode(test.input)
			if err != nil && !test.expectError {
				t.Fatal(err)
			}
			if err == nil && test.expectError {
				t.Fatalf("expect error, but got none")
			}
			if len(g.Files) != len(test.expected.Files) {
				t.Fatalf("test fail: %s", test.name)
			}
			for i := 0; i < len(g.Files); i++ {
				if !reflect.DeepEqual(test.expected.Files[i], g.Files[i]) {
					t.Fatalf("expected: %+v, actual: %+v", test.expected.Files[i], g.Files[i])
				}
			}
		})
	}
}

func checkMixedMode(t *testing.T, tests []struct {
	name     string
	input    []vers.MigrationFile
	expected []GroupEntry
},
) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries, err := ParseFilesMixedMode(test.input)
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
