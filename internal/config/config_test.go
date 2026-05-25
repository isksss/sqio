package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefault(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Query.Timeout != "30s" {
		t.Fatalf("unexpected timeout: %s", cfg.Query.Timeout)
	}
}

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
