// Package query reads, splits, and classifies SQL input.
package query

import (
	"errors"
	"io"
	"os"
	"strings"
)

// ErrMultipleInputs is returned when SQL is supplied through more than one
// supported source at the same time.
var ErrMultipleInputs = errors.New("specify only one of --sql, --file, or stdin")

// Source describes the mutually exclusive places where SQL can be read from.
type Source struct {
	SQL  string
	File string
	In   io.Reader
}

// Statement contains one SQL statement and the one-based line where it starts.
type Statement struct {
	SQL  string
	Line int
}

// Read loads SQL from Source, rejecting ambiguous input combinations so callers
// do not accidentally execute different text than intended.
func Read(src Source) (string, error) {
	count := 0
	if src.SQL != "" {
		count++
	}
	if src.File != "" {
		count++
	}
	hasStdin := hasInput(src.In)
	if hasStdin {
		count++
	}
	if count > 1 {
		return "", ErrMultipleInputs
	}
	switch {
	case src.SQL != "":
		return src.SQL, nil
	case src.File != "":
		b, err := os.ReadFile(src.File)
		return string(b), err
	case hasStdin:
		b, err := io.ReadAll(src.In)
		return string(b), err
	default:
		return "", nil
	}
}

// Statements splits SQL into executable statements without line metadata.
func Statements(sql string) []string {
	parsed := StatementsWithLine(sql)
	statements := make([]string, 0, len(parsed))
	for _, statement := range parsed {
		statements = append(statements, statement.SQL)
	}
	return statements
}

// StatementsWithLine splits SQL on semicolons that are outside quoted strings
// and comments, preserving the starting line for diagnostics.
func StatementsWithLine(sql string) []Statement {
	statements := []Statement{}
	start := 0
	startLine := 1
	line := 1
	state := scanNormal
	for i := 0; i < len(sql); i++ {
		if sql[i] == '\n' {
			line++
		}
		switch state {
		case scanNormal:
			switch {
			case sql[i] == '\'':
				state = scanSingleQuote
			case sql[i] == '"':
				state = scanDoubleQuote
			case sql[i] == '`':
				state = scanBacktick
			case sql[i] == '-' && i+1 < len(sql) && sql[i+1] == '-':
				state = scanLineComment
				i++
			case sql[i] == '/' && i+1 < len(sql) && sql[i+1] == '*':
				state = scanBlockComment
				i++
			case sql[i] == ';':
				statements = appendStatement(statements, sql[start:i], startLine)
				start = i + 1
				startLine = line
			}
		case scanSingleQuote:
			if sql[i] == '\'' {
				if i+1 < len(sql) && sql[i+1] == '\'' {
					i++
				} else {
					state = scanNormal
				}
			}
		case scanDoubleQuote:
			if sql[i] == '"' {
				if i+1 < len(sql) && sql[i+1] == '"' {
					i++
				} else {
					state = scanNormal
				}
			}
		case scanBacktick:
			if sql[i] == '`' {
				state = scanNormal
			}
		case scanLineComment:
			if sql[i] == '\n' {
				state = scanNormal
			}
		case scanBlockComment:
			if sql[i] == '*' && i+1 < len(sql) && sql[i+1] == '/' {
				state = scanNormal
				i++
			}
		}
	}
	statements = appendStatement(statements, sql[start:], startLine)
	return statements
}

// appendStatement trims raw SQL and appends it when it contains executable text.
func appendStatement(statements []Statement, raw string, startLine int) []Statement {
	line := startLine + leadingNewlines(raw)
	statement := strings.TrimSpace(raw)
	if statement != "" {
		statements = append(statements, Statement{SQL: statement, Line: line})
	}
	return statements
}

// leadingNewlines counts blank leading lines so diagnostics point at the first
// non-whitespace token in a statement.
func leadingNewlines(s string) int {
	count := 0
	for _, r := range s {
		switch r {
		case '\n':
			count++
		case ' ', '\t', '\r':
			continue
		default:
			return count
		}
	}
	return count
}

// scanState tracks which SQL lexical context the lightweight scanner is in.
type scanState int

const (
	// scanNormal reads executable SQL text.
	scanNormal scanState = iota
	// scanSingleQuote reads a single-quoted string literal.
	scanSingleQuote
	// scanDoubleQuote reads a double-quoted identifier or string.
	scanDoubleQuote
	// scanBacktick reads a MySQL-style quoted identifier.
	scanBacktick
	// scanLineComment reads a line comment that ends at the next newline.
	scanLineComment
	// scanBlockComment reads a block comment that ends at the next */ marker.
	scanBlockComment
)

// AnalysisText returns SQL with comments and literals scrubbed while preserving
// line breaks, making keyword-based safety checks less prone to false matches.
func AnalysisText(sql string) string {
	return scrubSQL(sql, true)
}

// CommentlessText returns SQL with comments scrubbed but literal contents kept,
// which supports checks that need string patterns such as LIKE wildcards.
func CommentlessText(sql string) string {
	return scrubSQL(sql, false)
}

// scrubSQL replaces comments and, optionally, literal contents with spaces while
// keeping byte length and line breaks stable for line-oriented diagnostics.
func scrubSQL(sql string, scrubLiterals bool) string {
	var b strings.Builder
	b.Grow(len(sql))
	state := scanNormal
	for i := 0; i < len(sql); i++ {
		ch := sql[i]
		switch state {
		case scanNormal:
			switch {
			case ch == '\'':
				state = scanSingleQuote
				b.WriteByte(scrubByte(ch, scrubLiterals))
			case ch == '"':
				state = scanDoubleQuote
				b.WriteByte(scrubByte(ch, scrubLiterals))
			case ch == '`':
				state = scanBacktick
				b.WriteByte(scrubByte(ch, scrubLiterals))
			case ch == '-' && i+1 < len(sql) && sql[i+1] == '-':
				state = scanLineComment
				b.WriteString("  ")
				i++
			case ch == '/' && i+1 < len(sql) && sql[i+1] == '*':
				state = scanBlockComment
				b.WriteString("  ")
				i++
			default:
				b.WriteByte(ch)
			}
		case scanSingleQuote:
			b.WriteByte(scrubByte(ch, scrubLiterals))
			if ch == '\'' {
				if i+1 < len(sql) && sql[i+1] == '\'' {
					i++
					b.WriteByte(scrubByte(sql[i], scrubLiterals))
				} else {
					state = scanNormal
				}
			}
		case scanDoubleQuote:
			b.WriteByte(scrubByte(ch, scrubLiterals))
			if ch == '"' {
				if i+1 < len(sql) && sql[i+1] == '"' {
					i++
					b.WriteByte(scrubByte(sql[i], scrubLiterals))
				} else {
					state = scanNormal
				}
			}
		case scanBacktick:
			b.WriteByte(scrubByte(ch, scrubLiterals))
			if ch == '`' {
				state = scanNormal
			}
		case scanLineComment:
			b.WriteByte(preserveNewline(ch))
			if ch == '\n' {
				state = scanNormal
			}
		case scanBlockComment:
			b.WriteByte(preserveNewline(ch))
			if ch == '*' && i+1 < len(sql) && sql[i+1] == '/' {
				state = scanNormal
				i++
				b.WriteByte(' ')
			}
		}
	}
	return b.String()
}

// scrubByte preserves newlines and replaces other bytes with spaces when
// scrubbing is enabled.
func scrubByte(ch byte, scrub bool) byte {
	if !scrub {
		return ch
	}
	return preserveNewline(ch)
}

// preserveNewline keeps line structure intact while hiding non-newline content.
func preserveNewline(ch byte) byte {
	if ch == '\n' {
		return '\n'
	}
	return ' '
}

// hasInput reports whether r should be treated as a provided SQL source.
// Non-stdin readers are considered intentional input.
func hasInput(r io.Reader) bool {
	if r == nil {
		return false
	}
	if file, ok := r.(*os.File); !ok || file != os.Stdin {
		return true
	}
	return stdinHasData()
}

// stdinHasData reports whether stdin is connected to a pipe or file rather than
// an interactive terminal.
func stdinHasData() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}
