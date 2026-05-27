package query

import (
	"strings"
	"testing"
)

// TestReadSQL verifies the behavior covered by this test helper or case.
func TestReadSQL(t *testing.T) {
	got, err := Read(Source{SQL: "select 1"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "select 1" {
		t.Fatalf("want select 1, got %q", got)
	}
}

// TestStatements verifies the behavior covered by this test helper or case.
func TestStatements(t *testing.T) {
	got := Statements("select 1; select 2;")
	if len(got) != 2 {
		t.Fatalf("want 2 statements, got %d", len(got))
	}
}

// TestStatementsIgnoresSemicolonInLiteralAndComment verifies the behavior covered by this test helper or case.
func TestStatementsIgnoresSemicolonInLiteralAndComment(t *testing.T) {
	got := Statements("select ';' as value; -- ;\nselect 'ok;still ok';")
	if len(got) != 2 {
		t.Fatalf("want 2 statements, got %#v", got)
	}
}

// TestStatementsWithLine verifies the behavior covered by this test helper or case.
func TestStatementsWithLine(t *testing.T) {
	got := StatementsWithLine("\nselect 1;\n\n-- comment\nselect 2;")
	if len(got) != 2 {
		t.Fatalf("want 2 statements, got %#v", got)
	}
	if got[0].Line != 2 || got[1].Line != 4 {
		t.Fatalf("unexpected statement lines: %#v", got)
	}
}

// TestReadMultipleInputs verifies the behavior covered by this test helper or case.
func TestReadMultipleInputs(t *testing.T) {
	_, err := Read(Source{SQL: "select 1", File: "query.sql", In: strings.NewReader("select 2")})
	if err != ErrMultipleInputs {
		t.Fatalf("want ErrMultipleInputs, got %v", err)
	}
}

// TestDangerous verifies the behavior covered by this test helper or case.
func TestDangerous(t *testing.T) {
	danger, ok := Dangerous("delete from users")
	if !ok {
		t.Fatal("expected dangerous query")
	}
	if danger.Type != "delete_without_where" {
		t.Fatalf("unexpected danger: %s", danger.Type)
	}
}

// TestDangerousIgnoresWhereInComment verifies the behavior covered by this test helper or case.
func TestDangerousIgnoresWhereInComment(t *testing.T) {
	danger, ok := Dangerous("delete from users /* where id = 1 */")
	if !ok {
		t.Fatal("expected dangerous query")
	}
	if danger.Type != "delete_without_where" {
		t.Fatalf("unexpected danger: %s", danger.Type)
	}
}

// TestMutating verifies the behavior covered by this test helper or case.
func TestMutating(t *testing.T) {
	if !Mutating("insert into users (name) values ('a')") {
		t.Fatal("expected mutating query")
	}
	if Mutating("select * from users") {
		t.Fatal("expected read query")
	}
}

// TestMutatingIgnoresKeywordInComment verifies the behavior covered by this test helper or case.
func TestMutatingIgnoresKeywordInComment(t *testing.T) {
	if Mutating("-- select only\ninsert into users (name) values ('a')") != true {
		t.Fatal("expected mutating query")
	}
}
