package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginCommands(t *testing.T) {
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "sqio-plugin-hello")
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\necho plugin \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	out, err := executeForTest("plugin", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "hello\t") {
		t.Fatalf("unexpected plugin list: %s", out)
	}
	out, err = executeForTest("plugin", "list", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"name": "hello"`) {
		t.Fatalf("unexpected plugin json: %s", out)
	}
	out, err = executeForTest("plugin", "run", "hello", "arg")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "plugin arg" {
		t.Fatalf("unexpected plugin run output: %s", out)
	}
	if _, err := executeForTest("plugin", "list", "--format", "yaml"); err == nil {
		t.Fatal("expected bad format error")
	}
	if _, err := executeForTest("plugin", "run", "../bad"); err == nil {
		t.Fatal("expected invalid plugin name error")
	}
}
