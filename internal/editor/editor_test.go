package editor

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSelectFallback verifies the behavior covered by this test helper or case.
func TestSelectFallback(t *testing.T) {
	t.Setenv("DBTUI_EDITOR", "")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	if got := Select(); got != "vi" {
		t.Fatalf("expected vi, got %s", got)
	}
}

// TestSelectPriority verifies the behavior covered by this test helper or case.
func TestSelectPriority(t *testing.T) {
	t.Setenv("DBTUI_EDITOR", "nvim")
	t.Setenv("VISUAL", "code")
	t.Setenv("EDITOR", "vim")
	if got := Select(); got != "nvim" {
		t.Fatalf("expected nvim, got %s", got)
	}
}

// TestEditUsesConfiguredEditor verifies the external editor integration with a
// deterministic test editor.
func TestEditUsesConfiguredEditor(t *testing.T) {
	dir := t.TempDir()
	editorPath := filepath.Join(dir, "editor.sh")
	if err := os.WriteFile(editorPath, []byte("#!/bin/sh\nprintf 'select 2' > \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DBTUI_EDITOR", editorPath)
	got, err := Edit("select 1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "select 2" {
		t.Fatalf("unexpected edited sql: %q", got)
	}
}
