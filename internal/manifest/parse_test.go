package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantRev  int64
		wantStem string
		wantMode Mode
		wantOK   bool
	}{
		{
			name:     "up.sql",
			input:    "0000001-schemas.up.sql",
			wantRev:  1,
			wantStem: "0000001-schemas",
			wantMode: ModeDefault,
			wantOK:   true,
		},
		{
			name:     "r.sql",
			input:    "0000002-refresh-stats.r.sql",
			wantRev:  2,
			wantStem: "0000002-refresh-stats",
			wantMode: ModeRepeatable,
			wantOK:   true,
		},
		{
			name:     "notx.sql",
			input:    "0000003-vacuum.notx.sql",
			wantRev:  3,
			wantStem: "0000003-vacuum",
			wantMode: ModeNoTx,
			wantOK:   true,
		},
		{
			name:     "rnotx.sql",
			input:    "0000004-rebuild-view.rnotx.sql",
			wantRev:  4,
			wantStem: "0000004-rebuild-view",
			wantMode: ModeRepeatableNoTx,
			wantOK:   true,
		},
		{
			name:     "max revision",
			input:    "9999999-last.up.sql",
			wantRev:  9999999,
			wantStem: "9999999-last",
			wantMode: ModeDefault,
			wantOK:   true,
		},
		{
			name:     "multi-dash name",
			input:    "0000001-create-users-table.up.sql",
			wantRev:  1,
			wantStem: "0000001-create-users-table",
			wantMode: ModeDefault,
			wantOK:   true,
		},
		{
			name:     "dots in name",
			input:    "0000001-a.b.c.up.sql",
			wantRev:  1,
			wantStem: "0000001-a.b.c",
			wantMode: ModeDefault,
			wantOK:   true,
		},
		{
			name:   "empty string",
			input:  "",
			wantOK: false,
		},
		{
			name:   "no revision prefix",
			input:  "schemas.up.sql",
			wantOK: false,
		},
		{
			name:   "revision too short",
			input:  "000001-name.up.sql",
			wantOK: false,
		},
		{
			name:   "revision too long",
			input:  "00000001-name.up.sql",
			wantOK: false,
		},
		{
			name:   "non-digit revision",
			input:  "abcdefg-name.up.sql",
			wantOK: false,
		},
		{
			name:   "underscore separator",
			input:  "0000001_name.up.sql",
			wantOK: false,
		},
		{
			name:   "down migration",
			input:  "0000001-name.down.sql",
			wantOK: false,
		},
		{
			name:   "no kind extension",
			input:  "0000001-name.sql",
			wantOK: false,
		},
		{
			name:   "unknown extension",
			input:  "0000001-name.up.sql.bak",
			wantOK: false,
		},
		{
			name:   "plain sql file",
			input:  "plain.sql",
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseFilename(tc.input)
			require.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				assert.Equal(t, tc.wantRev, got.revision)
				assert.Equal(t, tc.wantStem, got.stem)
				assert.Equal(t, tc.wantMode, got.mode)
			}
		})
	}
}

func TestKindToMode(t *testing.T) {
	tests := []struct {
		kind string
		want Mode
	}{
		{"up", ModeDefault},
		{"r", ModeRepeatable},
		{"notx", ModeNoTx},
		{"rnotx", ModeRepeatableNoTx},
		{"", ModeDefault},
		{"unknown", ModeDefault},
	}
	for _, tc := range tests {
		t.Run(tc.kind, func(t *testing.T) {
			assert.Equal(t, tc.want, kindToMode(tc.kind))
		})
	}
}
