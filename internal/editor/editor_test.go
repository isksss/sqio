package editor

import "testing"

func TestSelectFallback(t *testing.T) {
	t.Setenv("DBTUI_EDITOR", "")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	if got := Select(); got != "vi" {
		t.Fatalf("expected vi, got %s", got)
	}
}

func TestSelectPriority(t *testing.T) {
	t.Setenv("DBTUI_EDITOR", "nvim")
	t.Setenv("VISUAL", "code")
	t.Setenv("EDITOR", "vim")
	if got := Select(); got != "nvim" {
		t.Fatalf("expected nvim, got %s", got)
	}
}
