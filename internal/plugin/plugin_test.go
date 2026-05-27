package plugin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListAndRun(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	pluginPath := filepath.Join(dir, "sqio-plugin-hello")
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\necho hello \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "sqio-plugin-hello"), []byte("#!/bin/sh\necho duplicate\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sqio-plugin-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sqio-plugin-skip"), []byte("no exec"), 0o644); err != nil {
		t.Fatal(err)
	}
	plugins, err := List(dir + string(os.PathListSeparator) + other + string(os.PathListSeparator) + filepath.Join(dir, "missing"))
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 || plugins[0].Name != "hello" {
		t.Fatalf("unexpected plugins: %+v", plugins)
	}
	out, err := Run(context.Background(), "hello", []string{"world"}, dir).Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "hello world" {
		t.Fatalf("unexpected plugin output: %s", out)
	}
}

func TestValidateName(t *testing.T) {
	for _, name := range []string{"", "../bad", "-bad"} {
		if err := ValidateName(name); err == nil {
			t.Fatalf("expected invalid plugin name: %q", name)
		}
	}
	if err := ValidateName("ok"); err != nil {
		t.Fatal(err)
	}
}
