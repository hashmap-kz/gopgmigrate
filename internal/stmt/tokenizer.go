package stmt

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Token types
type TokenType int

const (
	TokenRaw TokenType = iota // Anything except comments & strings
	TokenSingleLineComment
	TokenBlockComment
	TokenString
	TokenDollarQuoted
	TokenSemicolon
	TokenEOF
)

// Token structure
type Token struct {
	Type  TokenType
	Value string
}

// Tokenizer structure
type Tokenizer struct {
	sql      string
	position int
}

// Read the next Unicode rune
func (t *Tokenizer) nextRune() (rune, int, bool) {
	if t.position >= len(t.sql) {
		return 0, 0, false
	}
	r, size := utf8.DecodeRuneInString(t.sql[t.position:])
	t.position += size
	return r, size, true
}

// Peek the next Unicode rune without consuming
func (t *Tokenizer) peekRune() (rune, int, bool) {
	if t.position >= len(t.sql) {
		return 0, 0, false
	}
	r, size := utf8.DecodeRuneInString(t.sql[t.position:])
	return r, size, true
}

// Peek the next two Unicode runes without consuming
func (t *Tokenizer) peekTwoRunes() (rune, rune, bool) {
	if t.position >= len(t.sql) {
		return 0, 0, false
	}
	savedPos := t.position

	var has bool
	r1, _, has := t.nextRune()
	r2, _, has := t.nextRune()

	t.position = savedPos
	return r1, r2, has
}

func IsIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func IsIdentTail(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// Read a single-line comment
func (t *Tokenizer) readSingleLineComment() string {
	var sb strings.Builder
	sb.WriteString("--")
	for {
		r, _, ok := t.nextRune()
		if !ok || r == '\n' || r == '\r' {
			sb.WriteRune(r)
			break
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// Read a block comment, handling nested comments
func (t *Tokenizer) readBlockComment() string {
	var sb strings.Builder
	sb.WriteString("/*")
	nested := 1
	for {
		r, _, ok := t.nextRune()
		if !ok {
			break
		}
		sb.WriteRune(r)

		if r == '/' {
			if next, _, ok := t.peekRune(); ok && next == '*' {
				t.nextRune()
				sb.WriteString("*")
				nested++
			}
		} else if r == '*' {
			if next, _, ok := t.peekRune(); ok && next == '/' {
				t.nextRune()
				sb.WriteString("/")
				nested--
				if nested == 0 {
					break
				}
			}
		}
	}
	return sb.String()
}

// Read a single-quoted string
func (t *Tokenizer) readString1() string {
	var sb strings.Builder
	sb.WriteRune('\'')
	for {
		r, _, ok := t.nextRune()
		if !ok {
			break
		}
		sb.WriteRune(r)
		if r == '\'' {
			// Handle escaped single quotes ('') for PostgreSQL
			if next, _, ok := t.peekRune(); ok && next == '\'' {
				t.nextRune()
				sb.WriteRune(next)
			} else {
				break
			}
		} else if r == '\\' { // Handle backslash escaping for ClickHouse
			if next, _, ok := t.peekRune(); ok && (next == '\'' || next == '\\') {
				t.nextRune()
				sb.WriteRune(next)
			}
		}
	}
	return sb.String()
}

// Read a double-quoted, or backtick-quoted string
func (t *Tokenizer) readString2(stop rune) string {
	var sb strings.Builder
	sb.WriteRune(stop)
	for {
		r, _, ok := t.nextRune()
		if !ok {
			break
		}
		sb.WriteRune(r)
		if r == stop {
			break
		}
	}
	return sb.String()
}

// Read a dollar-quoted string ($tag$...$tag$ or $$...$$)
func (t *Tokenizer) readDollarQuoted() (string, bool) {
	var sb strings.Builder
	tag, ok := t.readDollarTag()
	if !ok {
		return tag, false
	}
	sb.WriteString(tag)

	for {
		r, _, ok := t.nextRune()
		if !ok {
			break
		}
		sb.WriteRune(r)
		if strings.HasSuffix(sb.String(), tag) {
			break
		}
	}
	return sb.String(), true
}

// Read a dollar tag (e.g., $tag$ or $$)
func (t *Tokenizer) readDollarTag() (string, bool) {
	var sb strings.Builder
	sb.WriteRune('$')

	r, _, ok := t.peekRune()
	if !ok {
		return sb.String(), false
	}
	// just $$
	if r == '$' {
		t.nextRune()
		sb.WriteRune(r)
		return sb.String(), true
	}
	// anything, but NOT a dollar-quoted ident
	// ${table}
	// $1
	if !IsIdentStart(r) {
		return sb.String(), false
	}

	// finally, seems like a dollar-tag
	for {
		r, _, ok := t.peekRune()
		if !ok {
			break
		}

		// End of tag
		if r == '$' {
			t.nextRune()
			sb.WriteRune(r)
			return sb.String(), true
		}

		// If it's not a letter, number, or underscore, stop
		if !IsIdentTail(r) {
			break
		}
		t.nextRune()
		sb.WriteRune(r)
	}
	return sb.String(), true
}

// Read raw text (everything not handled specifically)
func (t *Tokenizer) readRaw() string {
	var sb strings.Builder
	for {
		r, _, ok := t.peekRune()
		isSpecial := r == '-' || r == '/' || r == '\'' || r == '$' || r == ';' || r == '"' || r == '`'
		if !ok || isSpecial {
			break
		}
		r, _, _ = t.nextRune()
		sb.WriteRune(r)
	}
	rawResult := sb.String()
	return rawResult
}

// Tokenize input SQL
func (t *Tokenizer) NextToken() Token {
	// End of input
	if t.position >= len(t.sql) {
		return Token{Type: TokenEOF}
	}

	// r, _, _ := t.peekRune()
	r1, r2, _ := t.peekTwoRunes()

	// Handle single-line comments
	if r1 == '-' {
		t.nextRune()
		if r2 == '-' {
			t.nextRune()
			return Token{Type: TokenSingleLineComment, Value: t.readSingleLineComment()}
		}
		return Token{Type: TokenRaw, Value: string(r1)}
	}

	// Handle block comments
	if r1 == '/' {
		t.nextRune()
		if r2 == '*' {
			t.nextRune()
			return Token{Type: TokenBlockComment, Value: t.readBlockComment()}
		}
		return Token{Type: TokenRaw, Value: string(r1)}
	}

	// Handle single-quoted strings
	if r1 == '\'' {
		t.nextRune()
		return Token{Type: TokenString, Value: t.readString1()}
	}
	// Handle double-quoted or backtick-quoted strings
	if r1 == '"' || r2 == '`' {
		t.nextRune()
		return Token{Type: TokenString, Value: t.readString2(r1)}
	}

	// Handle dollar-quoted strings
	if r1 == '$' {
		nextRune, _, _ := t.nextRune()
		dollarQuoted, ok := t.readDollarQuoted()
		if ok {
			return Token{Type: TokenDollarQuoted, Value: dollarQuoted}
		}
		return Token{Type: TokenRaw, Value: string(nextRune)}
	}

	if r1 == ';' {
		t.nextRune()
		return Token{Type: TokenSemicolon, Value: ";"}
	}

	// Handle raw text
	return Token{Type: TokenRaw, Value: t.readRaw()}
}

// parser

type Parser struct {
	tokens   []Token
	position int
}

// Get the current token
func (p *Parser) currentToken() Token {
	if p.position >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.position]
}

// Consume the current token and move to the next
func (p *Parser) consume() {
	p.position++
}

// Parse SQL statement
func (p *Parser) parseStatement() string {
	var sb strings.Builder

	for {
		token := p.currentToken()

		// End at EOF or semicolon
		if token.Type == TokenEOF || token.Value == ";" {
			break
		}

		sb.WriteString(token.Value)
		p.consume()
	}

	// Consume semicolon
	if p.currentToken().Value == ";" {
		sb.WriteString(";")
		p.consume()
	}

	return sb.String()
}

// Parse the entire SQL input
func (p *Parser) parseSQL() []string {
	var statements []string

	for p.currentToken().Type != TokenEOF {
		statement := p.parseStatement()
		if strings.TrimSpace(statement) != "" {
			statements = append(statements, statement)
		}
	}

	if len(statements) == 0 {
		return []string{""}
	}
	return statements
}

func SplitSQLStatements(sql string) ([]string, error) {
	// Tokenize input
	tokenizer := &Tokenizer{sql: sql}
	var tokens []Token
	for {
		token := tokenizer.NextToken()
		if token.Type == TokenEOF {
			break
		}
		tokens = append(tokens, token)
	}

	// Parse tokens
	parser := &Parser{tokens: tokens}
	statements := parser.parseSQL()

	return statements, nil
}
