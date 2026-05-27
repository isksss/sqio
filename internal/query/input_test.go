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

func TestStatementsIgnoresSemicolonInQuotedIdentifiersAndBlockComments(t *testing.T) {
	got := Statements("select `a;b` from t; /* ; */\nselect \"c;d\" from t;")
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

func TestDangerousUsesTokens(t *testing.T) {
	if danger, ok := Dangerous("delete from users where_note = 'not a where clause'"); !ok || danger.Type != "delete_without_where" {
		t.Fatalf("expected token-aware missing where, got %+v ok=%v", danger, ok)
	}
	if danger, ok := Dangerous("update users set note = 'where hidden'"); !ok || danger.Type != "update_without_where" {
		t.Fatalf("expected literal where to be ignored, got %+v ok=%v", danger, ok)
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

func TestAnalysisTextScrubsQuotedTextAndComments(t *testing.T) {
	sql := "select 'delete' as x, \"update\" as y, `drop` as z -- truncate\nfrom t /* drop database x */"
	got := AnalysisText(sql)
	for _, hidden := range []string{"delete", "update", "drop", "truncate", "database"} {
		if strings.Contains(strings.ToLower(got), hidden) {
			t.Fatalf("expected %q to be scrubbed from %q", hidden, got)
		}
	}
	if strings.Count(got, "\n") != 1 {
		t.Fatalf("expected newline preservation: %q", got)
	}
}

func TestTokens(t *testing.T) {
	tokens := Tokens("select id from users where name = 'drop database prod'")
	if !HasTokenSequence(tokens, "select", "id", "from", "users") || !HasToken(tokens, "where") {
		t.Fatalf("unexpected tokens: %#v", tokens)
	}
	if HasToken(tokens, "drop") || HasToken(tokens, "database") {
		t.Fatalf("literal keywords should be scrubbed: %#v", tokens)
	}
}

func TestReadNoInput(t *testing.T) {
	got, err := Read(Source{})
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("expected empty SQL, got %q", got)
	}
}

func TestStdinHasDataWithFile(t *testing.T) {
	oldStdin := os.Stdin
	t.Cleanup(func() { os.Stdin = oldStdin })
	path := filepath.Join(t.TempDir(), "stdin.sql")
	if err := os.WriteFile(path, []byte("select 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	os.Stdin = file
	if !stdinHasData() || !hasInput(os.Stdin) {
		t.Fatal("expected file-backed stdin to have data")
	}
}
