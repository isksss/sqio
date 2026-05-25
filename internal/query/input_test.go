package query

import (
	"strings"
	"testing"
)

func TestReadSQL(t *testing.T) {
	got, err := Read(Source{SQL: "select 1"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "select 1" {
		t.Fatalf("want select 1, got %q", got)
	}
}

func TestStatements(t *testing.T) {
	got := Statements("select 1; select 2;")
	if len(got) != 2 {
		t.Fatalf("want 2 statements, got %d", len(got))
	}
}

func TestReadMultipleInputs(t *testing.T) {
	_, err := Read(Source{SQL: "select 1", File: "query.sql", In: strings.NewReader("select 2")})
	if err != ErrMultipleInputs {
		t.Fatalf("want ErrMultipleInputs, got %v", err)
	}
}

func TestDangerous(t *testing.T) {
	danger, ok := Dangerous("delete from users")
	if !ok {
		t.Fatal("expected dangerous query")
	}
	if danger.Type != "delete_without_where" {
		t.Fatalf("unexpected danger: %s", danger.Type)
	}
}

func TestMutating(t *testing.T) {
	if !Mutating("insert into users (name) values ('a')") {
		t.Fatal("expected mutating query")
	}
	if Mutating("select * from users") {
		t.Fatal("expected read query")
	}
}
