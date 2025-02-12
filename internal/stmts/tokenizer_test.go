package stmts

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestSimple1(t *testing.T) {
	tests := []struct {
		name     string
		inputSQL string
		expected int
	}{
		{"simple-1", "select 1", 1},
		{"simple-2", "select 1;", 1},
		{"multilineNestedComment", multilineNestedComment, 1},
		{"q1", q1, 11},
		{"audit", audit_stmts, 40},
		{"placeholder", "${table_name}", 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			splitResult, cnt := checkSplit(strings.TrimSpace(test.inputSQL))
			if cnt != test.expected {
				t.Errorf("%s: expected %d statements, got: %d", test.name, test.expected, cnt)
			}
			if strings.TrimSpace(splitResult) != strings.TrimSpace(test.inputSQL) {
				os.WriteFile("expected.txt", []byte(strings.TrimSpace(test.inputSQL)), 0o755)
				os.WriteFile("actual.txt", []byte(strings.TrimSpace(splitResult)), 0o755)
				t.Errorf("Content not matching: %s, expected: [%s], actual: [%s]",
					test.name,
					strings.TrimSpace(test.inputSQL),
					strings.TrimSpace(splitResult),
				)
			}
		})
	}
}

func TestSimple2(t *testing.T) {
	// SELECT c, ascii(c)
	// FROM unnest(string_to_array(E'\n\ta', NULL)) AS t(c);

	tests := []struct {
		name     string
		inputSQL string
		expected []string
	}{
		{"simple-1", "select 1", []string{`select 1`}},
		{"simple-2", `$$ select 1 $$ ; $t$ 1 $t$`, []string{`$$ select 1 $$ ;`, ` $t$ 1 $t$`}},
		{"simple-3", `select E'\n'`, []string{`select E'\n'`}},
		{"simple-4", `select "1"`, []string{`select "1"`}},
		{"simple-5", `${table_name}; select 1;`, []string{`${table_name};`, ` select 1;`}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			splitResult := checkSplitStmt(strings.TrimSpace(test.inputSQL), test.expected)
			if !splitResult {
				t.Errorf("%s fail", test.name)
			}
		})
	}
}

// Test Tokenizer Functions

func TestNextRune(t *testing.T) {
	tokenizer := &Tokenizer{sql: "abc"}
	r, _, ok := tokenizer.nextRune()
	if !ok || r != 'a' {
		t.Errorf("Expected 'a', got '%c'", r)
	}
}

func TestPeekRune(t *testing.T) {
	tokenizer := &Tokenizer{sql: "abc"}
	r, _, ok := tokenizer.peekRune()
	if !ok || r != 'a' {
		t.Errorf("Expected 'a', got '%c'", r)
	}

	// position should not change
	nextRune, _, _ := tokenizer.nextRune()
	if nextRune != 'a' {
		t.Errorf("Expected 'a', got '%c'", nextRune)
	}
}

func TestPeekTwoRunes(t *testing.T) {
	tokenizer := &Tokenizer{sql: "abc"}
	r1, r2, ok := tokenizer.peekTwoRunes()
	if !ok || r1 != 'a' || r2 != 'b' {
		t.Errorf("Expected 'a', 'b', got '%c', '%c'", r1, r2)
	}

	// position should not change
	nextRune, _, _ := tokenizer.nextRune()
	if nextRune != 'a' {
		t.Errorf("Expected 'a', got '%c'", nextRune)
	}
}

func TestReadSingleLineComment(t *testing.T) {
	tokenizer := &Tokenizer{sql: "-- This is a comment\nNext line"}
	tokenizer.nextRune() // Move to '-'
	tokenizer.nextRune()
	comment := tokenizer.readSingleLineComment()
	expected := "-- This is a comment\n"
	if comment != expected {
		t.Errorf("Expected '%s', got '%s'", expected, comment)
	}
}

func TestReadBlockComment(t *testing.T) {
	tokenizer := &Tokenizer{sql: "/* This is a block comment */ Next"}
	tokenizer.nextRune() // Move to '/'
	tokenizer.nextRune()
	comment := tokenizer.readBlockComment()
	expected := "/* This is a block comment */"
	if comment != expected {
		t.Errorf("Expected '%s', got '%s'", expected, comment)
	}
}

func TestReadNestedBlockComment(t *testing.T) {
	tokenizer := &Tokenizer{sql: "/* This is a block comment /* This is a nested block comment */ */ Next"}
	tokenizer.nextRune() // Move to '/'
	tokenizer.nextRune()
	comment := tokenizer.readBlockComment()
	expected := "/* This is a block comment /* This is a nested block comment */ */"
	if comment != expected {
		t.Errorf("Expected '%s', got '%s'", expected, comment)
	}
}

func TestReadString(t *testing.T) {
	tokenizer := &Tokenizer{sql: "'Hello ''world''' next"}
	tokenizer.nextRune() // Move to quote
	str := tokenizer.readString('\'')
	expected := "'Hello ''world'''"
	if str != expected {
		t.Errorf("Expected '%s', got '%s'", expected, str)
	}
}

func TestReadDollarQuoted(t *testing.T) {
	tokenizer := &Tokenizer{sql: "$tag$Hello world$tag$ next"}
	tokenizer.nextRune() // Move to $
	str, ok := tokenizer.readDollarQuoted()
	expected := "$tag$Hello world$tag$"
	if !ok || str != expected {
		t.Errorf("Expected '%s', got '%s'", expected, str)
	}
}

func TestReadDollarTag(t *testing.T) {
	tokenizer := &Tokenizer{sql: "$tag$"}
	tokenizer.nextRune() // Move to $
	tag, ok := tokenizer.readDollarTag()
	expected := "$tag$"
	if !ok || tag != expected {
		t.Errorf("Expected '%s', got '%s'", expected, tag)
	}
}

func TestReadRaw(t *testing.T) {
	tokenizer := &Tokenizer{sql: "SELECT * FROM users;"}
	raw := tokenizer.readRaw()
	expected := "SELECT * FROM users"
	if raw != expected {
		t.Errorf("Expected '%s', got '%s'", expected, raw)
	}
}

func TestNextToken(t *testing.T) {
	tokenizer := &Tokenizer{sql: "-- Comment\nSELECT 'hello';"}
	token := tokenizer.NextToken()
	if token.Type != TokenSingleLineComment {
		t.Errorf("Expected TokenSingleLineComment, got %v", token.Type)
	}

	token = tokenizer.NextToken()
	if token.Type != TokenRaw {
		t.Errorf("Expected TokenRaw, got %v", token.Type)
	}

	token = tokenizer.NextToken()
	if token.Type != TokenString {
		t.Errorf("Expected TokenString, got %v", token.Type)
	}

	token = tokenizer.NextToken()
	if token.Type != TokenSemicolon {
		t.Errorf("Expected TokenSemicolon, got %v", token.Type)
	}
}

// Test Parser Functions

func TestParseStatement(t *testing.T) {
	tokens := []Token{
		{Type: TokenRaw, Value: "SELECT * FROM users"},
		{Type: TokenSemicolon, Value: ";"},
	}
	parser := &Parser{tokens: tokens}
	stmt := parser.parseStatement()
	expected := "SELECT * FROM users;"
	if stmt != expected {
		t.Errorf("Expected '%s', got '%s'", expected, stmt)
	}
}

func TestParseSQL(t *testing.T) {
	tokens := []Token{
		{Type: TokenRaw, Value: "SELECT * FROM users"},
		{Type: TokenSemicolon, Value: ";"},
		{Type: TokenRaw, Value: "INSERT INTO users (name) VALUES ('Alice')"},
		{Type: TokenSemicolon, Value: ";"},
	}
	parser := &Parser{tokens: tokens}
	statements := parser.parseSQL()
	expected := []string{
		"SELECT * FROM users;",
		"INSERT INTO users (name) VALUES ('Alice');",
	}
	if !reflect.DeepEqual(statements, expected) {
		t.Errorf("Expected %v, got %v", expected, statements)
	}
}

func TestSplitSQLStatements2(t *testing.T) {
	sql := `
	SELECT * FROM users;
	INSERT INTO users (name) VALUES ('Alice');
	`
	statements := splitTrimSpaces(sql)
	expected := []string{
		"SELECT * FROM users;",
		"INSERT INTO users (name) VALUES ('Alice');",
	}
	if !reflect.DeepEqual(statements, expected) {
		t.Errorf("Expected %v, got %v", expected, statements)
	}
}

// Utils

func splitTrimSpaces(sql string) []string {
	statements, err := SplitSQLStatements(sql)
	if err != nil {
		return []string{}
	}
	results := []string{}
	for _, elem := range statements {
		results = append(results, strings.TrimSpace(elem))
	}
	return results
}

func checkSplitStmt(inputSQL string, expected []string) bool {
	statements, err := SplitSQLStatements(inputSQL)
	if err != nil {
		return false
	}
	if len(expected) != len(statements) {
		return false
	}
	for i := 0; i < len(expected); i++ {
		if statements[i] != expected[i] {
			fmt.Printf("expected=[%s]; actual=[%s]", expected[i], statements[i])
			return false
		}
	}
	return true
}

func checkSplit(inputSQL string) (string, int) {
	statements, err := SplitSQLStatements(inputSQL)
	if err != nil {
		return "", -1
	}
	sb := strings.Builder{}
	for _, elem := range statements {
		sb.WriteString(elem)
	}
	return sb.String(), len(statements)
}

var q1 = `
-- Single-line multilineNestedComment
CREATE TABLE users (
	id SERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	emoji TEXT DEFAULT '🔥'
);

/* Multi-line
   multilineNestedComment */
INSERT INTO users (name) VALUES ('Alice');

/*
	/* nested multiline multilineNestedComment */
*/
SELECT 1024;

INSERT INTO users (name) VALUES ('O''Connor'); -- Handle escaped quote

-- Dollar-quoted function
CREATE FUNCTION hello() RETURNS TEXT AS $$
BEGIN
	RETURN 'Hello, 世界!';
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION another() RETURNS TEXT AS $tag$
BEGIN
	RETURN 'Another function';
END;
$tag$ LANGUAGE plpgsql;

-- Extended strings
INSERT INTO messages (text) VALUES (E'Hello\nNew Line\tTab');

INSERT INTO messages (text, name) VALUES ($1, $2);

-- select 1
select 2; -- select 3;
/* select 4; */
select 'select 5;';

--select 6;
select 7;
`

var multilineNestedComment = `
/* /*1*/ 2 /*3/*4*/*/ */
1
`
