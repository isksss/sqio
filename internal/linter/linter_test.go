package linter

import "testing"

func TestLintSelectStar(t *testing.T) {
	result := Lint("select * from users")
	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result.Issues))
	}
	if result.Issues[0].Rule != "select-star" {
		t.Fatalf("expected select-star, got %s", result.Issues[0].Rule)
	}
}

func TestLintIgnore(t *testing.T) {
	result := Lint("-- sqio:ignore select-star\nselect * from users")
	if len(result.Issues) != 0 {
		t.Fatalf("expected no issues, got %d", len(result.Issues))
	}
}

func TestLintDisable(t *testing.T) {
	result := Lint("select * from users", Options{Disable: []string{"select-star"}})
	if len(result.Issues) != 0 {
		t.Fatalf("expected no issues, got %d", len(result.Issues))
	}
}

func TestLintLevel(t *testing.T) {
	result := Lint("select * from users", Options{Level: "error"})
	if len(result.Issues) != 0 {
		t.Fatalf("expected warning to be filtered, got %d", len(result.Issues))
	}
}

func TestLintSafetyAndPerformanceRules(t *testing.T) {
	result := Lint("truncate users\nselect * from users where name like '%foo' or id = 1 or id = 2 or id = 3")
	if len(result.Issues) != 4 {
		t.Fatalf("expected 4 issues, got %#v", result.Issues)
	}
}

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

func TestLintKeywordCaseOptIn(t *testing.T) {
	result := Lint("select id from users", Options{Enable: []string{"keyword-case"}, Level: "info"})
	if len(result.Issues) != 1 || result.Issues[0].Rule != "keyword-case" {
		t.Fatalf("expected keyword-case issue, got %#v", result.Issues)
	}
}
