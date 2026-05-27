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

func TestDSNDuckDB(t *testing.T) {
	got, err := DSN(Connection{Driver: "duckdb", Database: "warehouse.duckdb"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "warehouse.duckdb" {
		t.Fatalf("unexpected dsn: %s", got)
	}
	if _, err := DSN(Connection{Driver: "duckdb"}); err == nil {
		t.Fatal("expected duckdb database error")
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

func TestDSNCompatibleAliases(t *testing.T) {
	got, err := DSN(Connection{Driver: "cockroach", Host: "db", Database: "app", User: "app"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "postgres://app") || !strings.Contains(got, "sslmode=disable") {
		t.Fatalf("unexpected cockroach dsn: %s", got)
	}
	got, err = DSN(Connection{Driver: "mariadb", Host: "db", Database: "app", User: "app"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "tcp(db:3306)/app") {
		t.Fatalf("unexpected mariadb dsn: %s", got)
	}
	got, err = DSN(Connection{Driver: "tidb", Host: "db", Database: "app", User: "app"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "tcp(db:3306)/app") {
		t.Fatalf("unexpected tidb dsn: %s", got)
	}
}

func TestDSNAdditionalDrivers(t *testing.T) {
	got, err := DSN(Connection{Driver: "sqlserver", Host: "db", Database: "app", User: "sa", Password: "sec@ret"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "sqlserver://sa:sec%40ret@db:1433?") || !strings.Contains(got, "database=app") {
		t.Fatalf("unexpected sqlserver dsn: %s", got)
	}
	got, err = DSN(Connection{Driver: "oracle", Host: "db", Database: "XEPDB1", User: "app", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "oracle://app:secret@db:1521/XEPDB1") {
		t.Fatalf("unexpected oracle dsn: %s", got)
	}
	got, err = DSN(Connection{Driver: "clickhouse", Host: "db", Database: "analytics", User: "default", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "clickhouse://default:secret@db:9000/analytics" {
		t.Fatalf("unexpected clickhouse dsn: %s", got)
	}
	if _, err := DSN(Connection{Driver: "oracle"}); err == nil {
		t.Fatal("expected oracle service name error")
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

func TestDSNDefaultsAndErrors(t *testing.T) {
	pg, err := DSN(Connection{Driver: "postgres", Database: "app"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pg, "localhost:5432") || !strings.Contains(pg, "sslmode=disable") {
		t.Fatalf("unexpected postgres default dsn: %s", pg)
	}
	my, err := DSN(Connection{Driver: "mysql", Database: "app"})
	if err != nil {
		t.Fatal(err)
	}
	if my != "tcp(localhost:3306)/app" {
		t.Fatalf("unexpected mysql default dsn: %s", my)
	}
	if _, err := DSN(Connection{Driver: "sqlite"}); err == nil {
		t.Fatal("expected sqlite database error")
	}
	if _, err := DSN(Connection{Driver: "bad"}); err == nil {
		t.Fatal("expected unsupported driver error")
	}
	explicit, err := DSN(Connection{DSN: "explicit"})
	if err != nil || explicit != "explicit" {
		t.Fatalf("unexpected explicit dsn: %s %v", explicit, err)
	}
}
