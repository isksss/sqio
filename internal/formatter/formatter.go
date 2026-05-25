package formatter

import (
	"regexp"
	"strings"
)

type Options struct {
	Dialect        string
	Indent         int
	KeywordCase    string
	IdentifierCase string
	LineWidth      int
}

var keywords = []string{
	"select", "from", "where", "insert", "into", "values", "update", "set", "delete",
	"join", "left", "right", "inner", "outer", "on", "group", "by", "order", "limit",
	"having", "returning", "create", "table", "alter", "drop", "and", "or", "as",
}

var clausePatterns = []string{
	`\bfrom\b`,
	`\bwhere\b`,
	`\bgroup\s+by\b`,
	`\border\s+by\b`,
	`\bhaving\b`,
	`\blimit\b`,
	`\breturning\b`,
}

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
