// Package linter implements sqio's SQL safety and style checks.
package linter

import (
	"strings"

	"github.com/isksss/sqio/internal/query"
)

// Issue describes one lint finding with enough metadata for text and structured
// output formats.
type Issue struct {
	Line     int    `json:"line"`
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// Result is the complete lint response for one SQL input.
type Result struct {
	Issues []Issue `json:"issues"`
}

// Options controls which lint rules are enabled, disabled, or filtered by
// severity.
type Options struct {
	Dialect string
	Level   string
	Enable  []string
	Disable []string
}

// Lint analyzes SQL text and returns style, safety, and performance findings.
// It ignores comments and string literals for checks that should only consider
// executable SQL.
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
		tokens := query.Tokens(statement.SQL)
		normalized := strings.ToLower(strings.Join(strings.Fields(query.AnalysisText(statement.SQL)), " "))
		commentless := strings.ToLower(strings.Join(strings.Fields(query.CommentlessText(statement.SQL)), " "))
		if query.HasTokenSequence(tokens, "delete", "from") && !query.HasToken(tokens, "where") && !ignored["delete-without-where"] && !disabled["delete-without-where"] {
			issues = appendIssue(issues, options.Level, Issue{Line: statement.Line, Rule: "delete-without-where", Severity: "error", Message: "DELETE without WHERE"})
		}
		if len(tokens) > 0 && tokens[0] == "update" && !query.HasToken(tokens, "where") && !ignored["update-without-where"] && !disabled["update-without-where"] {
			issues = appendIssue(issues, options.Level, Issue{Line: statement.Line, Rule: "update-without-where", Severity: "error", Message: "UPDATE without WHERE"})
		}
		if len(tokens) > 0 && tokens[0] == "truncate" && !ignored["truncate"] && !disabled["truncate"] {
			issues = appendIssue(issues, options.Level, Issue{Line: statement.Line, Rule: "truncate", Severity: "error", Message: "TRUNCATE is dangerous"})
		}
		if query.HasTokenSequence(tokens, "drop", "database") && !ignored["drop-database"] && !disabled["drop-database"] {
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
		for _, issue := range dialectIssues(options.Dialect, normalized, commentless, tokens, statement.Line) {
			if ignored[issue.Rule] || disabled[issue.Rule] {
				continue
			}
			issues = appendIssue(issues, options.Level, issue)
		}
	}
	return Result{Issues: issues}
}

// appendIssue appends issue only when its severity is at least the requested
// level, allowing callers to filter low-priority findings early.
func appendIssue(issues []Issue, level string, issue Issue) []Issue {
	if severityRank(issue.Severity) < severityRank(level) {
		return issues
	}
	return append(issues, issue)
}

// hasImplicitJoin reports comma-separated tables in a FROM clause, which is
// usually harder to read and easier to misuse than explicit JOIN syntax.
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

// hasCartesianJoin reports JOIN clauses that do not include ON or USING.
func hasCartesianJoin(line string) bool {
	return strings.Contains(line, " join ") && !strings.Contains(line, " on ") && !strings.Contains(line, " using ")
}

// hasNotInNull reports NOT IN expressions whose list contains NULL, a pattern
// that prevents matches under SQL's three-valued logic.
func hasNotInNull(line string) bool {
	if !strings.Contains(line, " not in ") {
		return false
	}
	return strings.Contains(line, "(null") || strings.Contains(line, ", null") || strings.Contains(line, " null,") || strings.Contains(line, " null)")
}

// hasLimitWithoutOrder reports LIMIT clauses that do not include ORDER BY.
func hasLimitWithoutOrder(line string) bool {
	return strings.Contains(line, " limit ") && !strings.Contains(line, " order by ")
}

func dialectIssues(dialect, normalized, commentless string, tokens []string, line int) []Issue {
	switch strings.ToLower(dialect) {
	case "postgres", "postgresql", "pgx", "cockroach", "cockroachdb":
		return postgresDialectIssues(normalized, commentless, line)
	case "mysql", "mariadb", "tidb":
		return mysqlDialectIssues(normalized, commentless, line)
	case "sqlite", "sqlite3":
		return sqliteDialectIssues(normalized, commentless, line)
	case "sqlserver", "mssql":
		return sqlServerDialectIssues(normalized, commentless, tokens, line)
	case "oracle":
		return oracleDialectIssues(normalized, commentless, tokens, line)
	case "duckdb":
		return duckDBDialectIssues(normalized, tokens, line)
	case "clickhouse", "ch":
		return clickHouseDialectIssues(normalized, tokens, line)
	default:
		return nil
	}
}

func postgresDialectIssues(normalized, commentless string, line int) []Issue {
	issues := []Issue{}
	if strings.Contains(commentless, "`") {
		issues = append(issues, Issue{Line: line, Rule: "postgres-backtick-identifier", Severity: "error", Message: "PostgreSQL uses double quotes for identifiers, not backticks"})
	}
	if hasMySQLLimitOffset(normalized) {
		issues = append(issues, Issue{Line: line, Rule: "postgres-limit-offset", Severity: "error", Message: "PostgreSQL uses LIMIT n OFFSET m instead of LIMIT m,n"})
	}
	return issues
}

func mysqlDialectIssues(normalized, _ string, line int) []Issue {
	issues := []Issue{}
	if strings.Contains(normalized, " ilike ") {
		issues = append(issues, Issue{Line: line, Rule: "mysql-ilike", Severity: "error", Message: "MySQL does not support ILIKE"})
	}
	if strings.Contains(normalized, " returning ") {
		issues = append(issues, Issue{Line: line, Rule: "mysql-returning", Severity: "warning", Message: "RETURNING support is not portable across MySQL versions"})
	}
	return issues
}

func sqliteDialectIssues(normalized, _ string, line int) []Issue {
	issues := []Issue{}
	if strings.Contains(normalized, " for update") {
		issues = append(issues, Issue{Line: line, Rule: "sqlite-for-update", Severity: "error", Message: "SQLite does not support FOR UPDATE"})
	}
	if strings.Contains(normalized, " ilike ") {
		issues = append(issues, Issue{Line: line, Rule: "sqlite-ilike", Severity: "error", Message: "SQLite does not support ILIKE"})
	}
	return issues
}

func sqlServerDialectIssues(normalized, commentless string, tokens []string, line int) []Issue {
	issues := []Issue{}
	if strings.Contains(commentless, "`") {
		issues = append(issues, Issue{Line: line, Rule: "sqlserver-backtick-identifier", Severity: "error", Message: "SQL Server uses brackets or double quotes for identifiers, not backticks"})
	}
	if query.HasToken(tokens, "limit") {
		issues = append(issues, Issue{Line: line, Rule: "sqlserver-limit", Severity: "error", Message: "SQL Server uses TOP or OFFSET/FETCH instead of LIMIT"})
	}
	if strings.Contains(normalized, " ilike ") {
		issues = append(issues, Issue{Line: line, Rule: "sqlserver-ilike", Severity: "error", Message: "SQL Server does not support ILIKE"})
	}
	if strings.Contains(normalized, " returning ") {
		issues = append(issues, Issue{Line: line, Rule: "sqlserver-returning", Severity: "error", Message: "SQL Server uses OUTPUT instead of RETURNING"})
	}
	return issues
}

func oracleDialectIssues(normalized, commentless string, tokens []string, line int) []Issue {
	issues := []Issue{}
	if strings.Contains(commentless, "`") {
		issues = append(issues, Issue{Line: line, Rule: "oracle-backtick-identifier", Severity: "error", Message: "Oracle uses double quotes for identifiers, not backticks"})
	}
	if query.HasToken(tokens, "limit") {
		issues = append(issues, Issue{Line: line, Rule: "oracle-limit", Severity: "error", Message: "Oracle uses FETCH FIRST or ROWNUM instead of LIMIT"})
	}
	if strings.Contains(normalized, " ilike ") {
		issues = append(issues, Issue{Line: line, Rule: "oracle-ilike", Severity: "error", Message: "Oracle does not support ILIKE"})
	}
	return issues
}

func duckDBDialectIssues(normalized string, tokens []string, line int) []Issue {
	issues := []Issue{}
	if strings.Contains(normalized, " for update") {
		issues = append(issues, Issue{Line: line, Rule: "duckdb-for-update", Severity: "error", Message: "DuckDB does not support FOR UPDATE"})
	}
	if len(tokens) > 0 && tokens[0] == "show" && query.HasToken(tokens, "tables") {
		issues = append(issues, Issue{Line: line, Rule: "duckdb-show-tables", Severity: "warning", Message: "Prefer information_schema or duckdb_tables() for portable DuckDB table metadata"})
	}
	return issues
}

func clickHouseDialectIssues(normalized string, tokens []string, line int) []Issue {
	issues := []Issue{}
	if query.HasToken(tokens, "returning") {
		issues = append(issues, Issue{Line: line, Rule: "clickhouse-returning", Severity: "error", Message: "ClickHouse does not support RETURNING"})
	}
	if strings.Contains(normalized, " for update") {
		issues = append(issues, Issue{Line: line, Rule: "clickhouse-for-update", Severity: "error", Message: "ClickHouse does not support FOR UPDATE"})
	}
	if len(tokens) > 0 && (tokens[0] == "update" || tokens[0] == "delete") {
		issues = append(issues, Issue{Line: line, Rule: "clickhouse-mutation", Severity: "warning", Message: "ClickHouse mutations use ALTER TABLE ... UPDATE/DELETE"})
	}
	return issues
}

func hasMySQLLimitOffset(normalized string) bool {
	idx := strings.LastIndex(normalized, " limit ")
	if idx < 0 {
		return false
	}
	tail := normalized[idx+len(" limit "):]
	if next := strings.Index(tail, " "); next >= 0 {
		tail = tail[:next]
	}
	return strings.Contains(tail, ",")
}

// hasLowercaseKeyword reports whether a supported SQL keyword appears in lower
// case on the analyzed line.
func hasLowercaseKeyword(line string) bool {
	for _, keyword := range []string{"select", "from", "where", "insert", "update", "delete", "join", "returning", "limit"} {
		if strings.Contains(line, keyword) {
			return true
		}
	}
	return false
}

// severityRank converts a severity label into an ordered value for filtering.
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
