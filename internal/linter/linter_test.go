package linter

import "testing"

// TestLintSelectStar verifies the behavior covered by this test helper or case.
func TestLintSelectStar(t *testing.T) {
	result := Lint("select * from users")
	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result.Issues))
	}
	if result.Issues[0].Rule != "select-star" {
		t.Fatalf("expected select-star, got %s", result.Issues[0].Rule)
	}
}

// TestLintIgnore verifies the behavior covered by this test helper or case.
func TestLintIgnore(t *testing.T) {
	result := Lint("-- sqio:ignore select-star\nselect * from users")
	if len(result.Issues) != 0 {
		t.Fatalf("expected no issues, got %d", len(result.Issues))
	}
}

// TestLintIgnoresCommentsAndStringLiterals verifies the behavior covered by this test helper or case.
func TestLintIgnoresCommentsAndStringLiterals(t *testing.T) {
	result := Lint("select 'select * from users' as sql -- select * from posts\n/*\nselect * from audit\n*/\nselect id from users")
	if len(result.Issues) != 0 {
		t.Fatalf("expected no issues, got %#v", result.Issues)
	}
}

// TestLintWhereInCommentDoesNotHideUnsafeDelete verifies the behavior covered by this test helper or case.
func TestLintWhereInCommentDoesNotHideUnsafeDelete(t *testing.T) {
	result := Lint("delete from users /* where id = 1 */")
	if len(result.Issues) != 1 || result.Issues[0].Rule != "delete-without-where" {
		t.Fatalf("expected delete-without-where, got %#v", result.Issues)
	}
}

// TestLintAllowsMultilineWhere verifies the behavior covered by this test helper or case.
func TestLintAllowsMultilineWhere(t *testing.T) {
	result := Lint("delete from users\nwhere id = 1;\nupdate users\nset name = 'alice'\nwhere id = 1;")
	if len(result.Issues) != 0 {
		t.Fatalf("expected no issues, got %#v", result.Issues)
	}
}

// TestLintDisable verifies the behavior covered by this test helper or case.
func TestLintDisable(t *testing.T) {
	result := Lint("select * from users", Options{Disable: []string{"select-star"}})
	if len(result.Issues) != 0 {
		t.Fatalf("expected no issues, got %d", len(result.Issues))
	}
}

// TestLintLevel verifies the behavior covered by this test helper or case.
func TestLintLevel(t *testing.T) {
	result := Lint("select * from users", Options{Level: "error"})
	if len(result.Issues) != 0 {
		t.Fatalf("expected warning to be filtered, got %d", len(result.Issues))
	}
}

// TestLintSafetyAndPerformanceRules verifies the behavior covered by this test helper or case.
func TestLintSafetyAndPerformanceRules(t *testing.T) {
	result := Lint("truncate users\nselect * from users where name like '%foo' or id = 1 or id = 2 or id = 3")
	if len(result.Issues) != 4 {
		t.Fatalf("expected 4 issues, got %#v", result.Issues)
	}
}

// TestLintJoinRules verifies the behavior covered by this test helper or case.
func TestLintJoinRules(t *testing.T) {
	result := Lint("select * from users, posts\nselect * from users join posts")
	rules := map[string]bool{}
	for _, issue := range result.Issues {
		rules[issue.Rule] = true
	}
	for _, rule := range []string{"implicit-join", "cartesian-join"} {
		if !rules[rule] {
			t.Fatalf("expected %s in %#v", rule, result.Issues)
		}
	}
}

// TestLintNullAndLimitRules verifies the behavior covered by this test helper or case.
func TestLintNullAndLimitRules(t *testing.T) {
	result := Lint("select id from users where status not in ('active', null)\nselect id from users limit 10")
	rules := map[string]bool{}
	for _, issue := range result.Issues {
		rules[issue.Rule] = true
	}
	for _, rule := range []string{"not-in-null", "limit-without-order"} {
		if !rules[rule] {
			t.Fatalf("expected %s in %#v", rule, result.Issues)
		}
	}
}

// TestLintLimitWithOrderBy verifies the behavior covered by this test helper or case.
func TestLintLimitWithOrderBy(t *testing.T) {
	result := Lint("select id from users\norder by id\nlimit 10")
	if len(result.Issues) != 0 {
		t.Fatalf("expected no issues, got %#v", result.Issues)
	}
}

// TestLintStatementRuleLineNumber verifies the behavior covered by this test helper or case.
func TestLintStatementRuleLineNumber(t *testing.T) {
	result := Lint("\n\nupdate users\nset name = 'alice';")
	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %#v", result.Issues)
	}
	if result.Issues[0].Rule != "update-without-where" || result.Issues[0].Line != 3 {
		t.Fatalf("unexpected issue: %#v", result.Issues[0])
	}
}

// TestLintKeywordCaseOptIn verifies the behavior covered by this test helper or case.
func TestLintKeywordCaseOptIn(t *testing.T) {
	result := Lint("select id from users", Options{Enable: []string{"keyword-case"}, Level: "info"})
	if len(result.Issues) != 1 || result.Issues[0].Rule != "keyword-case" {
		t.Fatalf("expected keyword-case issue, got %#v", result.Issues)
	}
}
