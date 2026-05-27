package query

import (
	"os"
	"path/filepath"
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

// TestReadFileAndReader verifies file and explicit reader sources.
func TestReadFileAndReader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "query.sql")
	if err := os.WriteFile(path, []byte("select 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Read(Source{File: path})
	if err != nil {
		t.Fatal(err)
	}
	if got != "select 1" {
		t.Fatalf("unexpected file sql: %q", got)
	}
	got, err = Read(Source{In: strings.NewReader("select 2")})
	if err != nil {
		t.Fatal(err)
	}
	if got != "select 2" {
		t.Fatalf("unexpected reader sql: %q", got)
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

// TestDangerousPatterns verifies destructive pattern classification.
func TestDangerousPatterns(t *testing.T) {
	cases := map[string]string{
		"truncate users":                 "truncate",
		"drop database prod":             "drop_database",
		"update users set x=1":           "update_without_where",
		"delete from users":              "delete_without_where",
		"delete from users where id = 1": "",
	}
	for sql, want := range cases {
		danger, ok := Dangerous(sql)
		if want == "" {
			if ok {
				t.Fatalf("expected safe query, got %+v", danger)
			}
			continue
		}
		if !ok || danger.Type != want {
			t.Fatalf("expected %s for %q, got %+v ok=%v", want, sql, danger, ok)
		}
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

// TestCommentlessTextPreservesLiteral verifies comment scrubbing can keep string
// literals for lint checks that need literal contents.
func TestCommentlessTextPreservesLiteral(t *testing.T) {
	got := CommentlessText("select 'a*b' /* hidden */")
	if !strings.Contains(got, "'a*b'") || strings.Contains(got, "hidden") {
		t.Fatalf("unexpected commentless text: %q", got)
	}
}
