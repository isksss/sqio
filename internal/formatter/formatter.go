// Package formatter contains sqio's lightweight SQL formatter.
package formatter

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Options controls SQL formatting behavior requested by CLI flags or config.
// Unsupported options are retained so config can evolve without changing the
// public service boundary.
type Options struct {
	Dialect        string
	Indent         int
	KeywordCase    string
	IdentifierCase string
	LineWidth      int
}

// keywords lists the tokens whose case can be normalized by Format.
var keywords = []string{
	"select", "from", "where", "insert", "into", "values", "update", "set", "delete",
	"join", "left", "right", "inner", "outer", "on", "group", "by", "order", "limit",
	"having", "returning", "create", "table", "alter", "drop", "and", "or", "as",
	"distinct", "union", "all", "offset", "fetch", "for", "ilike", "with", "case",
	"when", "then", "else", "end",
}

var keywordSet = makeKeywordSet(keywords)

// Format returns a normalized SQL string using sqio's conservative formatter.
// It tokenizes SQL before changing keyword/identifier case, so comments and
// literal contents are not modified.
func Format(sql string, opts Options) string {
	formatted := renderTokens(lex(strings.TrimSpace(sql)), opts)
	if formatted != "" && !strings.HasSuffix(formatted, "\n") {
		formatted += "\n"
	}
	return formatted
}

type tokenKind int

const (
	tokenWord tokenKind = iota
	tokenPunct
	tokenLiteral
	tokenComment
)

type token struct {
	kind tokenKind
	text string
}

func lex(sql string) []token {
	tokens := []token{}
	for i := 0; i < len(sql); {
		r, width := utf8.DecodeRuneInString(sql[i:])
		switch {
		case unicode.IsSpace(r):
			i += width
		case isWordRune(r):
			start := i
			i += width
			for i < len(sql) {
				next, nextWidth := utf8.DecodeRuneInString(sql[i:])
				if !isWordRune(next) {
					break
				}
				i += nextWidth
			}
			tokens = append(tokens, token{kind: tokenWord, text: sql[start:i]})
		case r == '\'' || r == '"' || r == '`':
			literal, next := readQuoted(sql, i, byte(r))
			tokens = append(tokens, token{kind: tokenLiteral, text: literal})
			i = next
		case r == '-' && i+1 < len(sql) && sql[i+1] == '-':
			start := i
			i += 2
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			tokens = append(tokens, token{kind: tokenComment, text: strings.TrimSpace(sql[start:i])})
		case r == '/' && i+1 < len(sql) && sql[i+1] == '*':
			start := i
			i += 2
			for i+1 < len(sql) && !(sql[i] == '*' && sql[i+1] == '/') {
				i++
			}
			if i+1 < len(sql) {
				i += 2
			}
			tokens = append(tokens, token{kind: tokenComment, text: strings.TrimSpace(sql[start:i])})
		default:
			tokens = append(tokens, token{kind: tokenPunct, text: string(r)})
			i += width
		}
	}
	return tokens
}

func readQuoted(sql string, start int, quote byte) (string, int) {
	for i := start + 1; i < len(sql); i++ {
		if sql[i] == quote {
			if quote == '\'' && i+1 < len(sql) && sql[i+1] == '\'' {
				i++
				continue
			}
			if quote == '"' && i+1 < len(sql) && sql[i+1] == '"' {
				i++
				continue
			}
			return sql[start : i+1], i + 1
		}
	}
	return sql[start:], len(sql)
}

func renderTokens(tokens []token, opts Options) string {
	indent := opts.Indent
	if indent <= 0 {
		indent = 2
	}
	lines := []string{}
	current := strings.Builder{}
	parenDepth := 0
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.kind == tokenComment {
			flushLine(&lines, &current)
			lines = append(lines, tok.text)
			continue
		}
		if startsClause(tokens, i) && strings.TrimSpace(current.String()) != "" {
			flushLine(&lines, &current)
		}
		text := formatToken(tok, opts)
		if tok.text == ")" && parenDepth > 0 {
			parenDepth--
		}
		writeToken(&current, text, tok)
		if tok.text == "(" {
			parenDepth++
		}
		if tok.text == "," && parenDepth == 0 {
			flushLine(&lines, &current)
		}
	}
	flushLine(&lines, &current)
	return indentLines(lines, indent)
}

func formatToken(tok token, opts Options) string {
	if tok.kind != tokenWord {
		return tok.text
	}
	lower := strings.ToLower(tok.text)
	if keywordSet[lower] {
		return applyCase(tok.text, opts.KeywordCase)
	}
	if opts.IdentifierCase != "" {
		return applyCase(tok.text, opts.IdentifierCase)
	}
	return tok.text
}

func applyCase(text, mode string) string {
	switch strings.ToLower(mode) {
	case "upper":
		return strings.ToUpper(text)
	case "lower":
		return strings.ToLower(text)
	default:
		return text
	}
}

func startsClause(tokens []token, i int) bool {
	if tokens[i].kind != tokenWord {
		return false
	}
	word := strings.ToLower(tokens[i].text)
	switch word {
	case "from", "where", "having", "limit", "offset", "returning", "union":
		return true
	case "group", "order":
		return i+1 < len(tokens) && strings.EqualFold(tokens[i+1].text, "by")
	default:
		return false
	}
}

func writeToken(current *strings.Builder, text string, tok token) {
	raw := current.String()
	trimmed := strings.TrimRight(raw, " ")
	current.Reset()
	current.WriteString(trimmed)
	if needsSpaceBefore(trimmed, tok) {
		current.WriteByte(' ')
	}
	current.WriteString(text)
	if tok.kind == tokenWord || tok.kind == tokenLiteral || tok.text == "," {
		current.WriteByte(' ')
	}
}

func needsSpaceBefore(current string, tok token) bool {
	if current == "" {
		return false
	}
	if tok.text == "," || tok.text == ")" || tok.text == ";" || tok.text == "." {
		return false
	}
	if strings.HasSuffix(current, "(") || strings.HasSuffix(current, ".") {
		return false
	}
	return true
}

func flushLine(lines *[]string, current *strings.Builder) {
	line := strings.TrimSpace(current.String())
	current.Reset()
	if line == "" {
		return
	}
	*lines = append(*lines, line)
}

func indentLines(lines []string, indent int) string {
	pad := strings.Repeat(" ", indent)
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" && !strings.HasPrefix(strings.TrimSpace(lines[i]), "--") {
			lines[i] = pad + strings.TrimSpace(lines[i])
		}
	}
	return strings.Join(lines, "\n")
}

func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func makeKeywordSet(words []string) map[string]bool {
	set := map[string]bool{}
	for _, word := range words {
		set[word] = true
	}
	return set
}
