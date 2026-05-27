package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadDefault verifies the behavior covered by this test helper or case.
func TestLoadDefault(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Query.Timeout != "30s" {
		t.Fatalf("unexpected timeout: %s", cfg.Query.Timeout)
	}
}

// TestLoadTOML verifies the behavior covered by this test helper or case.
func TestLoadTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	err := os.WriteFile(path, []byte("[query]\ntimeout = \"5s\"\nmax_rows = 10\nformat = \"json\"\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Query.Timeout != "5s" || cfg.Query.MaxRows != 10 || cfg.Query.Format != "json" {
		t.Fatalf("unexpected query config: %+v", cfg.Query)
	}
}

// TestLoadConnectionEnvPassword verifies the behavior covered by this test helper or case.
func TestLoadConnectionEnvPassword(t *testing.T) {
	t.Setenv("SQIO_TEST_PASSWORD", "secret")
	path := filepath.Join(t.TempDir(), "config.toml")
	err := os.WriteFile(path, []byte("[[connections]]\nname = \"local\"\ndriver = \"sqlite\"\ndatabase = \"test.db\"\npassword = \"env:SQIO_TEST_PASSWORD\"\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := cfg.Connection("local")
	if err != nil {
		t.Fatal(err)
	}
	if conn.Password != "secret" {
		t.Fatalf("unexpected password expansion")
	}
}

// TestLoadSSHTunnel verifies the behavior covered by this test helper or case.
func TestLoadSSHTunnel(t *testing.T) {
	t.Setenv("SQIO_SSH_PASSWORD", "ssh-secret")
	path := filepath.Join(t.TempDir(), "config.toml")
	err := os.WriteFile(path, []byte(`[[connections]]
name = "local"
driver = "postgres"
host = "db.internal"
database = "app"

[connections.ssh_tunnel]
enabled = true
host = "bastion"
port = 22
user = "deploy"
password = "env:SQIO_SSH_PASSWORD"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := cfg.Connection("local")
	if err != nil {
		t.Fatal(err)
	}
	if !conn.SSHTunnel.Enabled || conn.SSHTunnel.Password != "ssh-secret" {
		t.Fatalf("unexpected tunnel config: %+v", conn.SSHTunnel)
	}
}

// TestLoadGlobalAndNearestLocal verifies the behavior covered by this test helper or case.
func TestLoadGlobalAndNearestLocal(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	globalPath := DefaultPath()
	if err := os.MkdirAll(filepath.Dir(globalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(globalPath, []byte(`[query]
timeout = "10s"
format = "json"

[[connections]]
name = "global"
driver = "sqlite"
database = "global.db"

[[connections]]
name = "shared"
driver = "sqlite"
database = "global-shared.db"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(dir, "project")
	child := filepath.Join(project, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "sqio.toml"), []byte(`[query]
timeout = "5s"

[[connections]]
name = "shared"
driver = "sqlite"
database = "local-shared.db"

[[connections]]
name = "local"
driver = "sqlite"
database = "local.db"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	current, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(current)
	if err := os.Chdir(child); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Query.Timeout != "5s" || cfg.Query.Format != "json" {
		t.Fatalf("unexpected query config: %+v", cfg.Query)
	}
	if conn, err := cfg.Connection("global"); err != nil || conn.Database != "global.db" {
		t.Fatalf("unexpected global connection: %+v err=%v", conn, err)
	}
	if conn, err := cfg.Connection("shared"); err != nil || conn.Database != "local-shared.db" {
		t.Fatalf("unexpected shared connection: %+v err=%v", conn, err)
	}
	if conn, err := cfg.Connection("local"); err != nil || conn.Database != "local.db" {
		t.Fatalf("unexpected local connection: %+v err=%v", conn, err)
	}
}

// TestLoadLocalDoesNotApplyOutsideTree verifies the behavior covered by this test helper or case.
func TestLoadLocalDoesNotApplyOutsideTree(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	project := filepath.Join(dir, "project")
	sibling := filepath.Join(dir, "sibling")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "sqio.toml"), []byte("[query]\ntimeout = \"5s\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	current, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(current)
	if err := os.Chdir(sibling); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Query.Timeout != "30s" {
		t.Fatalf("unexpected timeout: %s", cfg.Query.Timeout)
	}
}

// TestLoadExplicitConfigSkipsAutoDiscovery verifies the behavior covered by this test helper or case.
func TestLoadExplicitConfigSkipsAutoDiscovery(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	globalPath := DefaultPath()
	if err := os.MkdirAll(filepath.Dir(globalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(globalPath, []byte("[query]\ntimeout = \"10s\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(dir, "sqio.toml")
	if err := os.WriteFile(local, []byte("[query]\ntimeout = \"5s\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	explicit := filepath.Join(dir, "explicit.toml")
	if err := os.WriteFile(explicit, []byte("[query]\nformat = \"json\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	current, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(current)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(explicit)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Query.Timeout != "30s" || cfg.Query.Format != "json" {
		t.Fatalf("unexpected query config: %+v", cfg.Query)
	}
}
