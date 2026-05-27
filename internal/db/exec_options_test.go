package db

import (
	"context"
	"path/filepath"
	"testing"
)

// TestExecuteMaxRows verifies the behavior covered by this test helper or case.
func TestExecuteMaxRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	_, err := Execute(context.Background(), Config{Driver: "sqlite", DSN: path}, `
create table users (id integer primary key);
insert into users (id) values (1), (2), (3);
`, ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(context.Background(), Config{Driver: "sqlite", DSN: path}, `select id from users order by id`, ExecuteOptions{MaxRows: 2})
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows, got %d", result.RowCount)
	}
}

// TestExecuteExplainSQLite verifies the behavior covered by this test helper or case.
func TestExecuteExplainSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	_, err := Execute(context.Background(), Config{Driver: "sqlite", DSN: path}, `create table users (id integer primary key);`, ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(context.Background(), Config{Driver: "sqlite", DSN: path}, `select id from users`, ExecuteOptions{Explain: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount == 0 {
		t.Fatal("expected explain rows")
	}
}

// TestExecuteTransaction verifies the behavior covered by this test helper or case.
func TestExecuteTransaction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	result, err := Execute(context.Background(), Config{Driver: "sqlite", DSN: path}, `
create table users (id integer primary key);
insert into users (id) values (1);
`, ExecuteOptions{Transaction: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 1 {
		t.Fatalf("expected affected row count, got %d", result.RowCount)
	}
}
