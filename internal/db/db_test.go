package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestExecuteSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	_, err := Execute(context.Background(), Config{Driver: "sqlite", DSN: path}, `
create table users (id integer primary key, name text);
insert into users (name) values ('alice');
select id, name from users;
`, ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(context.Background(), Config{Driver: "sqlite", DSN: path}, `select id, name from users;`, ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 1 {
		t.Fatalf("expected 1 row, got %d", result.RowCount)
	}
	if result.Rows[0][1] != "alice" {
		t.Fatalf("expected alice, got %#v", result.Rows[0][1])
	}
}

func TestMetadataSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	_, err := Execute(context.Background(), Config{Driver: "sqlite", DSN: path}, `
create table users (id integer primary key, name text not null, email text);
`, ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	schema, err := Metadata(context.Background(), Config{Driver: "sqlite", DSN: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(schema.Tables))
	}
	if schema.Tables[0].Name != "users" {
		t.Fatalf("expected users table, got %s", schema.Tables[0].Name)
	}
	if len(schema.Tables[0].Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(schema.Tables[0].Columns))
	}
}
