package formatter

import "testing"

func TestFormatKeywordCase(t *testing.T) {
	got := Format("select id from users", Options{KeywordCase: "upper"})
	want := "SELECT id\n  FROM users\n"
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestFormatIdempotent(t *testing.T) {
	once := Format("select id from users where id = 1", Options{KeywordCase: "upper", Indent: 2})
	twice := Format(once, Options{KeywordCase: "upper", Indent: 2})
	if once != twice {
		t.Fatalf("expected idempotent format:\nonce=%q\ntwice=%q", once, twice)
	}
}
