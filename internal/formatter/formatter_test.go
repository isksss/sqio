package formatter

import (
	"strings"
	"testing"
)

// TestFormatKeywordCase verifies the behavior covered by this test helper or case.
func TestFormatKeywordCase(t *testing.T) {
	got := Format("select id from users", Options{KeywordCase: "upper"})
	want := "SELECT id\n  FROM users\n"
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

// TestFormatIdempotent verifies the behavior covered by this test helper or case.
func TestFormatIdempotent(t *testing.T) {
	once := Format("select id from users where id = 1", Options{KeywordCase: "upper", Indent: 2})
	twice := Format(once, Options{KeywordCase: "upper", Indent: 2})
	if once != twice {
		t.Fatalf("expected idempotent format:\nonce=%q\ntwice=%q", once, twice)
	}
}

func TestFormatAdditionalKeywords(t *testing.T) {
	got := Format("select distinct id from users union all select id from archived offset 10", Options{KeywordCase: "upper"})
	for _, want := range []string{"SELECT DISTINCT", "UNION ALL", "OFFSET"} {
		if !contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
	}
}

func TestFormatDoesNotRewriteLiteralsOrComments(t *testing.T) {
	got := Format("select 'select from' as sql -- select from\nfrom users", Options{KeywordCase: "upper"})
	if !strings.Contains(got, "'select from'") || !strings.Contains(got, "-- select from") {
		t.Fatalf("literal/comment should be preserved: %q", got)
	}
	if strings.Contains(got, "'SELECT FROM'") {
		t.Fatalf("literal keyword should not be uppercased: %q", got)
	}
}

func TestFormatIdentifierCase(t *testing.T) {
	got := Format("select UserID, DisplayName from Users", Options{KeywordCase: "upper", IdentifierCase: "lower"})
	for _, want := range []string{"SELECT userid", "displayname", "FROM users"} {
		if !contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
	}
}

func TestFormatClauseAndCommaLayout(t *testing.T) {
	got := Format("select id, name from users where id = 1 order by name limit 10", Options{KeywordCase: "upper", Indent: 4})
	want := "SELECT id,\n    name\n    FROM users\n    WHERE id = 1\n    ORDER BY name\n    LIMIT 10\n"
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestFormatQuotedIdentifiersAndBlockComments(t *testing.T) {
	got := Format(`select "User""Name", users.name, count(*) from users /* from stays */ where name = 'it''s ok';`, Options{KeywordCase: "upper", IdentifierCase: "lower"})
	for _, want := range []string{`"User""Name"`, "users.name", "count (*)", "/* from stays */", "'it''s ok'", ";"} {
		if !contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
	}
	if contains(got, `"user""name"`) || contains(got, "/* FROM STAYS */") {
		t.Fatalf("quoted identifiers and comments should be preserved: %q", got)
	}
}

func TestFormatUnterminatedQuotedText(t *testing.T) {
	got := Format("select 'unterminated", Options{KeywordCase: "upper"})
	if got != "SELECT 'unterminated\n" {
		t.Fatalf("unexpected unterminated literal format: %q", got)
	}
}

func TestFormatLowercaseKeywords(t *testing.T) {
	got := Format("SELECT ID FROM USERS", Options{KeywordCase: "lower", IdentifierCase: "lower"})
	if got != "select id\n  from users\n" {
		t.Fatalf("unexpected lower-case format: %q", got)
	}
}

func TestFormatDefaultCaseModePreservesText(t *testing.T) {
	got := Format("select UserID from Users", Options{KeywordCase: "preserve", IdentifierCase: "preserve"})
	if got != "select UserID\n  from Users\n" {
		t.Fatalf("unexpected preserved format: %q", got)
	}
	if got := Format("   ", Options{}); got != "" {
		t.Fatalf("expected empty SQL to stay empty, got %q", got)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
