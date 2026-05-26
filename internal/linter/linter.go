package linter

import (
	"strings"

	"github.com/isksss/sqio/internal/query"
)

type Issue struct {
	Line     int    `json:"line"`
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type Result struct {
	Issues []Issue `json:"issues"`
}

type Options struct {
	Dialect string
	Level   string
	Enable  []string
	Disable []string
}

func Lint(sql string, opts ...Options) Result {
	options := Options{Level: "warning"}
	if len(opts) > 0 {
		options = opts[0]
	}
	disabled := map[string]bool{}
	for _, rule := range options.Disable {
		disabled[rule] = true
	}
	enabled := map[string]bool{}
	for _, rule := range options.Enable {
		enabled[rule] = true
	}
	lines := strings.Split(sql, "\n")
	analysisLines := strings.Split(query.AnalysisText(sql), "\n")
	ignored := map[string]bool{}
	issues := []Issue{}
	for i, line := range lines {
		rawNormalized := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(rawNormalized, "-- sqio:ignore ") {
			ignored[strings.TrimSpace(strings.TrimPrefix(rawNormalized, "-- sqio:ignore "))] = true
			continue
		}
		analysisLine := analysisLines[i]
		normalized := strings.ToLower(strings.TrimSpace(analysisLine))
		if strings.Contains(normalized, "select *") && !ignored["select-star"] && !disabled["select-star"] {
			issues = appendIssue(issues, options.Level, Issue{Line: i + 1, Rule: "select-star", Severity: "warning", Message: "avoid SELECT *"})
		}
		if strings.Count(normalized, " or ") >= 3 && !ignored["or-abuse"] && !disabled["or-abuse"] {
			issues = appendIssue(issues, options.Level, Issue{Line: i + 1, Rule: "or-abuse", Severity: "warning", Message: "many OR conditions can be hard to optimize"})
		}
		if hasImplicitJoin(normalized) && !ignored["implicit-join"] && !disabled["implicit-join"] {
			issues = appendIssue(issues, options.Level, Issue{Line: i + 1, Rule: "implicit-join", Severity: "warning", Message: "avoid comma-style implicit joins"})
		}
		if hasCartesianJoin(normalized) && !ignored["cartesian-join"] && !disabled["cartesian-join"] {
			issues = appendIssue(issues, options.Level, Issue{Line: i + 1, Rule: "cartesian-join", Severity: "error", Message: "JOIN without ON/USING may create a cartesian product"})
		}
		if (enabled["keyword-case"] || options.Dialect != "") && hasLowercaseKeyword(analysisLine) && !ignored["keyword-case"] && !disabled["keyword-case"] {
			issues = appendIssue(issues, options.Level, Issue{Line: i + 1, Rule: "keyword-case", Severity: "info", Message: "SQL keywords should use configured case"})
		}
	}
	for _, statement := range query.StatementsWithLine(sql) {
		normalized := strings.ToLower(strings.Join(strings.Fields(query.AnalysisText(statement.SQL)), " "))
		commentless := strings.ToLower(strings.Join(strings.Fields(query.CommentlessText(statement.SQL)), " "))
		if strings.HasPrefix(normalized, "delete from ") && !strings.Contains(normalized, " where ") && !ignored["delete-without-where"] && !disabled["delete-without-where"] {
			issues = appendIssue(issues, options.Level, Issue{Line: statement.Line, Rule: "delete-without-where", Severity: "error", Message: "DELETE without WHERE"})
		}
		if strings.HasPrefix(normalized, "update ") && !strings.Contains(normalized, " where ") && !ignored["update-without-where"] && !disabled["update-without-where"] {
			issues = appendIssue(issues, options.Level, Issue{Line: statement.Line, Rule: "update-without-where", Severity: "error", Message: "UPDATE without WHERE"})
		}
		if strings.HasPrefix(normalized, "truncate ") && !ignored["truncate"] && !disabled["truncate"] {
			issues = appendIssue(issues, options.Level, Issue{Line: statement.Line, Rule: "truncate", Severity: "error", Message: "TRUNCATE is dangerous"})
		}
		if strings.HasPrefix(normalized, "drop database ") && !ignored["drop-database"] && !disabled["drop-database"] {
			issues = appendIssue(issues, options.Level, Issue{Line: statement.Line, Rule: "drop-database", Severity: "error", Message: "DROP DATABASE is dangerous"})
		}
		if hasNotInNull(normalized) && !ignored["not-in-null"] && !disabled["not-in-null"] {
			issues = appendIssue(issues, options.Level, Issue{Line: statement.Line, Rule: "not-in-null", Severity: "error", Message: "NOT IN with NULL never matches as expected"})
		}
		if hasLimitWithoutOrder(normalized) && !ignored["limit-without-order"] && !disabled["limit-without-order"] {
			issues = appendIssue(issues, options.Level, Issue{Line: statement.Line, Rule: "limit-without-order", Severity: "warning", Message: "LIMIT without ORDER BY is nondeterministic"})
		}
		if strings.Contains(commentless, " like '%") && !ignored["leading-wildcard-like"] && !disabled["leading-wildcard-like"] {
			issues = appendIssue(issues, options.Level, Issue{Line: statement.Line, Rule: "leading-wildcard-like", Severity: "warning", Message: "leading wildcard LIKE can prevent index use"})
		}
	}
	return Result{Issues: issues}
}

func appendIssue(issues []Issue, level string, issue Issue) []Issue {
	if severityRank(issue.Severity) < severityRank(level) {
		return issues
	}
	return append(issues, issue)
}

func hasImplicitJoin(line string) bool {
	fromIdx := strings.Index(line, " from ")
	if strings.HasPrefix(line, "from ") {
		fromIdx = 0
	}
	if fromIdx < 0 {
		return false
	}
	tail := line[fromIdx:]
	whereIdx := strings.Index(tail, " where ")
	if whereIdx >= 0 {
		tail = tail[:whereIdx]
	}
	return strings.Contains(tail, ",")
}

func hasCartesianJoin(line string) bool {
	return strings.Contains(line, " join ") && !strings.Contains(line, " on ") && !strings.Contains(line, " using ")
}

func hasNotInNull(line string) bool {
	if !strings.Contains(line, " not in ") {
		return false
	}
	return strings.Contains(line, "(null") || strings.Contains(line, ", null") || strings.Contains(line, " null,") || strings.Contains(line, " null)")
}

func hasLimitWithoutOrder(line string) bool {
	return strings.Contains(line, " limit ") && !strings.Contains(line, " order by ")
}

func hasLowercaseKeyword(line string) bool {
	for _, keyword := range []string{"select", "from", "where", "insert", "update", "delete", "join"} {
		if strings.Contains(line, keyword) {
			return true
		}
	}
	return false
}

func severityRank(severity string) int {
	switch strings.ToLower(severity) {
	case "error":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 2
	}
}
