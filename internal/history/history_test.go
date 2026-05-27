package history

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAppendAndList verifies the behavior covered by this test helper or case.
func TestAppendAndList(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "history.db"))
	err := store.Append(context.Background(), Entry{SQL: "select 1", Connection: "local", ElapsedMS: 10, Success: true, RowCount: 1, Driver: "sqlite"})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := store.List(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].SQL != "select 1" {
		t.Fatalf("unexpected sql: %s", entries[0].SQL)
	}
	if !entries[0].Success || entries[0].RowCount != 1 || entries[0].Driver != "sqlite" {
		t.Fatalf("unexpected execution details: %+v", entries[0])
	}
}

func TestHistoryDefaultsAndLimit(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), "env-history.db")
	t.Setenv("SQIO_HISTORY_PATH", envPath)
	if store := New(""); store.path != envPath {
		t.Fatalf("expected env history path, got %s", store.path)
	}
	t.Setenv("SQIO_HISTORY_PATH", "")
	store := New(filepath.Join(t.TempDir(), "history.db"))
	if err := store.Append(context.Background(), Entry{SQL: "select 1"}); err != nil {
		t.Fatal(err)
	}
	entries, err := store.List(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ExecutedAt.IsZero() {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	if DefaultPath() == "" {
		t.Fatal("expected default path")
	}
}

func TestListWithOptionsAndUpdates(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "history.db"))
	if err := store.Append(context.Background(), Entry{SQL: "select * from users", Connection: "local", Tags: "report"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Append(context.Background(), Entry{SQL: "select * from orders", Connection: "warehouse", Favorite: true, Tags: "finance"}); err != nil {
		t.Fatal(err)
	}
	entries, err := store.ListWithOptions(context.Background(), ListOptions{Search: "orders", Connection: "warehouse", Favorite: true, Tags: "finance"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !strings.Contains(entries[0].SQL, "orders") {
		t.Fatalf("unexpected filtered entries: %+v", entries)
	}
	if err := store.SetFavorite(context.Background(), entries[0].ID, false); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTags(context.Background(), entries[0].ID, "audited"); err != nil {
		t.Fatal(err)
	}
	entry, err := store.Get(context.Background(), entries[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Favorite || entry.Tags != "audited" {
		t.Fatalf("unexpected updated entry: %+v", entry)
	}
	if _, err := store.Get(context.Background(), 999); err == nil {
		t.Fatal("expected missing history entry error")
	}
	if err := store.SetFavorite(context.Background(), 999, true); err == nil {
		t.Fatal("expected missing favorite update error")
	}
}

func TestHistoryMigratesLegacySchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`create table history (
id integer primary key autoincrement,
sql text not null,
connection text not null,
elapsed_ms integer not null,
executed_at text not null,
tags text not null default '',
favorite integer not null default 0
)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`insert into history (sql, connection, elapsed_ms, executed_at, tags, favorite) values ('select 1', 'legacy', 3, '2024-01-01T00:00:00Z', '', 0)`); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	store := New(path)
	entries, err := store.List(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !entries[0].Success || entries[0].RowCount != 0 {
		t.Fatalf("unexpected migrated entry: %+v", entries)
	}
	if err := store.Append(context.Background(), Entry{SQL: "select broken", Error: "syntax error"}); err != nil {
		t.Fatal(err)
	}
	entries, err = store.List(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Success || entries[0].Error != "syntax error" {
		t.Fatalf("unexpected appended failure: %+v", entries)
	}
}

func TestHistoryOpenErrors(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := New(filepath.Join(filePath, "history.db"))
	if err := store.Append(context.Background(), Entry{SQL: "select 1"}); err == nil {
		t.Fatal("expected history directory error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store = New(filepath.Join(dir, "canceled.db"))
	if _, err := store.List(ctx, 1); err == nil {
		t.Fatal("expected canceled context error")
	}
}
