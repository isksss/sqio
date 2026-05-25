package linter

import (
	"strings"
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
	ignored := map[string]bool{}
	issues := []Issue{}
	for i, line := range lines {
		normalized := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(normalized, "-- sqio:ignore ") {
			ignored[strings.TrimSpace(strings.TrimPrefix(normalized, "-- sqio:ignore "))] = true
			continue
		}
		if strings.Contains(normalized, "select *") && !ignored["select-star"] && !disabled["select-star"] {
			issues = appendIssue(issues, options.Level, Issue{Line: i + 1, Rule: "select-star", Severity: "warning", Message: "avoid SELECT *"})
		}
		if strings.HasPrefix(normalized, "delete from ") && !strings.Contains(normalized, " where ") && !ignored["delete-without-where"] && !disabled["delete-without-where"] {
			issues = appendIssue(issues, options.Level, Issue{Line: i + 1, Rule: "delete-without-where", Severity: "error", Message: "DELETE without WHERE"})
		}
		if strings.HasPrefix(normalized, "update ") && !strings.Contains(normalized, " where ") && !ignored["update-without-where"] && !disabled["update-without-where"] {
			issues = appendIssue(issues, options.Level, Issue{Line: i + 1, Rule: "update-without-where", Severity: "error", Message: "UPDATE without WHERE"})
		}
		if strings.HasPrefix(normalized, "truncate ") && !ignored["truncate"] && !disabled["truncate"] {
			issues = appendIssue(issues, options.Level, Issue{Line: i + 1, Rule: "truncate", Severity: "error", Message: "TRUNCATE is dangerous"})
		}
		if strings.HasPrefix(normalized, "drop database ") && !ignored["drop-database"] && !disabled["drop-database"] {
			issues = appendIssue(issues, options.Level, Issue{Line: i + 1, Rule: "drop-database", Severity: "error", Message: "DROP DATABASE is dangerous"})
		}
		if strings.Contains(normalized, " like '%") && !ignored["leading-wildcard-like"] && !disabled["leading-wildcard-like"] {
			issues = appendIssue(issues, options.Level, Issue{Line: i + 1, Rule: "leading-wildcard-like", Severity: "warning", Message: "leading wildcard LIKE can prevent index use"})
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
		if (enabled["keyword-case"] || options.Dialect != "") && hasLowercaseKeyword(line) && !ignored["keyword-case"] && !disabled["keyword-case"] {
			issues = appendIssue(issues, options.Level, Issue{Line: i + 1, Rule: "keyword-case", Severity: "info", Message: "SQL keywords should use configured case"})
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
