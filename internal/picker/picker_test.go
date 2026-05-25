package picker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSQLFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.sql"), []byte("select 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("select 2"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := SQLFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || filepath.Base(files[0]) != "a.sql" {
		t.Fatalf("unexpected files: %#v", files)
	}
}

func TestPickFallback(t *testing.T) {
	got, err := Pick([]string{"a.sql", "b.sql"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "a.sql" {
		t.Fatalf("expected first option, got %s", got)
	}
}
