// Package formatter contains sqio's lightweight SQL formatter.
package formatter

import (
	"regexp"
	"strings"
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
}

// clausePatterns identifies major SQL clauses that should start on their own
// line after whitespace normalization.
var clausePatterns = []string{
	`\bfrom\b`,
	`\bwhere\b`,
	`\bgroup\s+by\b`,
	`\border\s+by\b`,
	`\bhaving\b`,
	`\blimit\b`,
	`\breturning\b`,
}

// Format returns a normalized SQL string using sqio's conservative formatter.
// It keeps comments on their original lines, changes keyword case when asked,
// and appends a trailing newline for stable CLI and file output.
func Format(sql string, opts Options) string {
	formatted := normalizeSpacePreservingLines(strings.TrimSpace(sql))
	for _, keyword := range keywords {
		replacement := keyword
		if strings.EqualFold(opts.KeywordCase, "upper") {
			replacement = strings.ToUpper(keyword)
		}
		formatted = replaceWord(formatted, keyword, replacement)
	}
	for _, pattern := range clausePatterns {
		formatted = newlineBeforeClause(formatted, pattern)
	}
	formatted = indentContinuation(formatted, opts.Indent)
	if formatted != "" && !strings.HasSuffix(formatted, "\n") {
		formatted += "\n"
	}
	return formatted
}

// normalizeSpacePreservingLines collapses repeated whitespace within code
// lines while leaving full-line SQL comments untouched.
func normalizeSpacePreservingLines(input string) string {
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	return strings.Join(lines, "\n")
}

// replaceWord replaces a keyword token without disturbing surrounding
// punctuation such as commas, parentheses, and semicolons.
func replaceWord(input, old, replacement string) string {
	fields := strings.Fields(input)
	for i, field := range fields {
		trimmed := strings.Trim(field, "(),;")
		if strings.EqualFold(trimmed, old) {
			fields[i] = strings.Replace(field, trimmed, replacement, 1)
		}
	}
	return strings.Join(fields, " ")
}

// newlineBeforeClause inserts a line break before a matching SQL clause unless
// the clause is already separated by an existing newline.
func newlineBeforeClause(input, pattern string) string {
	re := regexp.MustCompile(`(?i)\s+(` + pattern + `)`)
	return re.ReplaceAllStringFunc(input, func(match string) string {
		trimmed := strings.TrimSpace(match)
		if strings.Contains(match, "\n") {
			return match
		}
		return "\n" + trimmed
	})
}

// indentContinuation indents every non-comment line after the first by the
// configured width, defaulting to two spaces when the option is unset.
func indentContinuation(input string, indent int) string {
	if indent <= 0 {
		indent = 2
	}
	pad := strings.Repeat(" ", indent)
	lines := strings.Split(input, "\n")
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" && !strings.HasPrefix(strings.TrimSpace(lines[i]), "--") {
			lines[i] = pad + strings.TrimSpace(lines[i])
		}
	}
	return strings.Join(lines, "\n")
}
