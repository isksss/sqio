package history

import (
	"context"
	"path/filepath"
	"testing"
)

// TestAppendAndList verifies the behavior covered by this test helper or case.
func TestAppendAndList(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "history.db"))
	err := store.Append(context.Background(), Entry{SQL: "select 1", Connection: "local", ElapsedMS: 10})
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
}
