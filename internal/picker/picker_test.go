package picker

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSQLFiles verifies the behavior covered by this test helper or case.
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

// TestPickFallback verifies the behavior covered by this test helper or case.
func TestPickFallback(t *testing.T) {
	got, err := Pick([]string{"a.sql", "b.sql"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "a.sql" {
		t.Fatalf("expected first option, got %s", got)
	}
}

func TestPickEmpty(t *testing.T) {
	if _, err := Pick(nil); err == nil {
		t.Fatal("expected empty picker error")
	}
}

func TestSQLFilesSkipsHiddenDirectoriesAndSorts(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden", "hidden.sql"), []byte("select 0"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.SQL"), []byte("select 2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.sql"), []byte("select 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := SQLFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || filepath.Base(files[0]) != "a.sql" || filepath.Base(files[1]) != "b.SQL" {
		t.Fatalf("unexpected sorted files: %#v", files)
	}
}

func TestSQLFilesMissingRoot(t *testing.T) {
	if _, err := SQLFiles(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected missing root error")
	}
}

func TestPickUsesFZFSelection(t *testing.T) {
	dir := t.TempDir()
	fzfPath := filepath.Join(dir, "fzf")
	if err := os.WriteFile(fzfPath, []byte("#!/bin/sh\nprintf 'b.sql\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	got, err := Pick([]string{"a.sql", "b.sql"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "b.sql" {
		t.Fatalf("expected fzf selection, got %s", got)
	}
}
