package query

import (
	"errors"
	"io"
	"os"
	"strings"
)

var ErrMultipleInputs = errors.New("specify only one of --sql, --file, or stdin")

type Source struct {
	SQL  string
	File string
	In   io.Reader
}

type Statement struct {
	SQL  string
	Line int
}

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

func Statements(sql string) []string {
	parsed := StatementsWithLine(sql)
	statements := make([]string, 0, len(parsed))
	for _, statement := range parsed {
		statements = append(statements, statement.SQL)
	}
	return statements
}

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

func appendStatement(statements []Statement, raw string, startLine int) []Statement {
	line := startLine + leadingNewlines(raw)
	statement := strings.TrimSpace(raw)
	if statement != "" {
		statements = append(statements, Statement{SQL: statement, Line: line})
	}
	return statements
}

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

type scanState int

const (
	scanNormal scanState = iota
	scanSingleQuote
	scanDoubleQuote
	scanBacktick
	scanLineComment
	scanBlockComment
)

func AnalysisText(sql string) string {
	return scrubSQL(sql, true)
}

func CommentlessText(sql string) string {
	return scrubSQL(sql, false)
}

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

func scrubByte(ch byte, scrub bool) byte {
	if !scrub {
		return ch
	}
	return preserveNewline(ch)
}

func preserveNewline(ch byte) byte {
	if ch == '\n' {
		return '\n'
	}
	return ' '
}

func hasInput(r io.Reader) bool {
	if r == nil {
		return false
	}
	if file, ok := r.(*os.File); !ok || file != os.Stdin {
		return true
	}
	return stdinHasData()
}

func stdinHasData() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}
