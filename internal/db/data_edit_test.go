package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestDataEditRows(t *testing.T) {
	cfg := Config{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "test.db")}
	if _, err := Execute(context.Background(), cfg, "create table users (id integer primary key, name text, status text)", ExecuteOptions{}); err != nil {
		t.Fatal(err)
	}
	affected, err := InsertRow(context.Background(), cfg, "users", map[string]string{"name": "alice", "status": "new"})
	if err != nil {
		t.Fatal(err)
	}
	if affected != 1 {
		t.Fatalf("expected insert affected 1, got %d", affected)
	}
	affected, err = UpdateRows(context.Background(), cfg, "users", map[string]string{"status": "active"}, "name = 'alice'")
	if err != nil {
		t.Fatal(err)
	}
	if affected != 1 {
		t.Fatalf("expected update affected 1, got %d", affected)
	}
	result, err := Execute(context.Background(), cfg, "select status from users where name = 'alice'", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows[0][0] != "active" {
		t.Fatalf("unexpected updated status: %+v", result)
	}
	affected, err = DeleteRows(context.Background(), cfg, "users", "name = 'alice'")
	if err != nil {
		t.Fatal(err)
	}
	if affected != 1 {
		t.Fatalf("expected delete affected 1, got %d", affected)
	}
}

func TestDataEditValidation(t *testing.T) {
	cfg := Config{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "test.db")}
	if _, err := InsertRow(context.Background(), cfg, "", map[string]string{"name": "alice"}); err == nil {
		t.Fatal("expected missing table error")
	}
	if _, err := InsertRow(context.Background(), cfg, "users", nil); err == nil {
		t.Fatal("expected missing values error")
	}
	if _, err := UpdateRows(context.Background(), cfg, "", map[string]string{"name": "alice"}, "id = 1"); err == nil {
		t.Fatal("expected update missing table error")
	}
	if _, err := UpdateRows(context.Background(), cfg, "users", nil, "id = 1"); err == nil {
		t.Fatal("expected update missing values error")
	}
	if _, err := UpdateRows(context.Background(), cfg, "users", map[string]string{"name": "alice"}, ""); err == nil {
		t.Fatal("expected missing where error")
	}
	if _, err := DeleteRows(context.Background(), cfg, "", "id = 1"); err == nil {
		t.Fatal("expected delete missing table error")
	}
	if _, err := DeleteRows(context.Background(), cfg, "users", ""); err == nil {
		t.Fatal("expected missing delete where error")
	}
	if _, err := InsertRow(context.Background(), cfg, "missing", map[string]string{"name": "alice"}); err == nil {
		t.Fatal("expected insert execution error")
	}
	if _, err := UpdateRows(context.Background(), cfg, "missing", map[string]string{"name": "alice"}, "id = 1"); err == nil {
		t.Fatal("expected update execution error")
	}
	if _, err := DeleteRows(context.Background(), cfg, "missing", "id = 1"); err == nil {
		t.Fatal("expected delete execution error")
	}
}

func TestDataEditWithAliasDriverPlaceholders(t *testing.T) {
	oldOpen := openConnection
	t.Cleanup(func() { openConnection = oldOpen })
	path := filepath.Join(t.TempDir(), "test.db")
	sqliteCfg := Config{Driver: "sqlite", DSN: path}
	if _, err := Execute(context.Background(), sqliteCfg, "create table users (id integer primary key, name text)", ExecuteOptions{}); err != nil {
		t.Fatal(err)
	}
	openConnection = func(ctx context.Context, cfg Config) (*sql.DB, string, error) {
		conn, _, err := Open(ctx, sqliteCfg)
		return conn, cfg.Driver, err
	}
	for _, driver := range []string{"mysql", "pgx"} {
		if _, err := InsertRow(context.Background(), Config{Driver: driver}, "users", map[string]string{"name": driver}); err != nil {
			t.Fatalf("%s insert: %v", driver, err)
		}
		if _, err := UpdateRows(context.Background(), Config{Driver: driver}, "users", map[string]string{"name": driver + "-updated"}, "name = '"+driver+"'"); err != nil {
			t.Fatalf("%s update: %v", driver, err)
		}
		if _, err := DeleteRows(context.Background(), Config{Driver: driver}, "users", "name = '"+driver+"-updated'"); err != nil {
			t.Fatalf("%s delete: %v", driver, err)
		}
	}
}
