package db

import (
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
)

// TestDSNSQLite verifies the behavior covered by this test helper or case.
func TestDSNSQLite(t *testing.T) {
	got, err := DSN(Connection{Driver: "sqlite", Database: "test.db"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "test.db" {
		t.Fatalf("unexpected dsn: %s", got)
	}
}

// TestDSNPostgres verifies the behavior covered by this test helper or case.
func TestDSNPostgres(t *testing.T) {
	got, err := DSN(Connection{Driver: "postgres", Host: "localhost", Database: "app", User: "app", Password: "secret", SSLMode: "require"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "postgres://app:secret@localhost:5432/app?") || !strings.Contains(got, "sslmode=require") {
		t.Fatalf("unexpected dsn: %s", got)
	}
}

// TestDSNMySQL verifies the behavior covered by this test helper or case.
func TestDSNMySQL(t *testing.T) {
	got, err := DSN(Connection{Driver: "mysql", Host: "localhost", Database: "app", User: "app", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "app:secret@tcp(localhost:3306)/app" {
		t.Fatalf("unexpected dsn: %s", got)
	}
}

// TestDSNMySQLEscapesSpecialCharacters verifies MySQL DSN fields are escaped by
// the driver config formatter instead of string concatenation.
func TestDSNMySQLEscapesSpecialCharacters(t *testing.T) {
	got, err := DSN(Connection{Driver: "mysql", Host: "localhost", Database: "app/name?x=1", User: "app", Password: "sec@ret:word"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := mysql.ParseDSN(got)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.User != "app" || parsed.Passwd != "sec@ret:word" || parsed.DBName != "app/name?x=1" {
		t.Fatalf("unexpected parsed dsn: %#v", parsed)
	}
}
