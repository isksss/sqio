package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
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

func TestTimeoutDurationAndDefaultTOML(t *testing.T) {
	cfg := Default()
	timeout, err := TimeoutDuration(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if timeout.String() != "30s" {
		t.Fatalf("unexpected timeout: %s", timeout)
	}
	if !strings.Contains(DefaultTOML(), `theme = "dark"`) {
		t.Fatal("default toml missing theme")
	}
}

func TestValidate(t *testing.T) {
	cfg := Default()
	cfg.Query.Timeout = "bad"
	cfg.Query.MaxRows = -1
	cfg.Query.Format = "xml"
	cfg.Connections = []Connection{
		{Name: "dup", Driver: "sqlite"},
		{Name: "dup", Driver: "unsupported"},
		{Name: "ssh", Driver: "postgres", DSN: "postgres://example/db", SSHTunnel: SSHTunnel{Enabled: true, KeepAlive: "bad", ReconnectAttempts: -1, JumpHost: "jump"}},
	}
	issues := Validate(cfg)
	got := map[string]bool{}
	for _, issue := range issues {
		got[issue.Path] = true
	}
	for _, path := range []string{
		"query.timeout",
		"query.max_rows",
		"query.format",
		"connections[0].database",
		"connections[1].name",
		"connections[1].driver",
		"connections[2].ssh_tunnel",
		"connections[2].ssh_tunnel.host",
		"connections[2].ssh_tunnel.user",
		"connections[2].ssh_tunnel.keepalive",
		"connections[2].ssh_tunnel.reconnect_attempts",
		"connections[2].ssh_tunnel.jump_user",
	} {
		if !got[path] {
			t.Fatalf("expected issue for %s in %+v", path, issues)
		}
	}
	if issues := Validate(Default()); len(issues) != 0 {
		t.Fatalf("default config should be valid: %+v", issues)
	}
	valid := Default()
	valid.Connections = []Connection{
		{Name: "cockroach", Driver: "cockroachdb"},
		{Name: "mariadb", Driver: "mariadb"},
		{Name: "tidb", Driver: "tidb"},
		{Name: "sqlserver", Driver: "sqlserver"},
		{Name: "oracle", Driver: "oracle"},
		{Name: "clickhouse", Driver: "clickhouse"},
		{Name: "duckdb", Driver: "duckdb"},
	}
	if issues := Validate(valid); len(issues) != 0 {
		t.Fatalf("compatible aliases should be valid: %+v", issues)
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

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("SQIO_THEME", "light")
	t.Setenv("SQIO_EDITOR", "nvim")
	t.Setenv("SQIO_QUERY_TIMEOUT", "5s")
	t.Setenv("SQIO_QUERY_FORMAT", "json")
	t.Setenv("SQIO_QUERY_MAX_ROWS", "25")
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme != "light" || cfg.Editor != "nvim" || cfg.Query.Timeout != "5s" || cfg.Query.Format != "json" || cfg.Query.MaxRows != 25 {
		t.Fatalf("unexpected env config: %+v", cfg)
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

func TestLoadConnectionFilePassword(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "password.txt")
	if err := os.WriteFile(secretPath, []byte("file-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.toml")
	err := os.WriteFile(path, []byte("[[connections]]\nname = \"local\"\ndriver = \"sqlite\"\ndatabase = \"test.db\"\npassword = \"file:"+secretPath+"\"\n"), 0o644)
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
	if conn.Password != "file-secret" {
		t.Fatalf("unexpected file password expansion: %q", conn.Password)
	}
}

func TestLoadConnectionFilePasswordError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	err := os.WriteFile(path, []byte("[[connections]]\nname = \"local\"\ndriver = \"sqlite\"\ndatabase = \"test.db\"\npassword = \"file:\"\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected file secret error")
	}
}

// TestLoadSSHTunnel verifies the behavior covered by this test helper or case.
func TestLoadSSHTunnel(t *testing.T) {
	t.Setenv("SQIO_SSH_PASSWORD", "ssh-secret")
	t.Setenv("SQIO_SSH_JUMP_PASSWORD", "jump-secret")
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
known_hosts = "/tmp/sqio_known_hosts"
keepalive = "30s"
reconnect = true
reconnect_attempts = 3
jump_host = "jump"
jump_port = 2022
jump_user = "jump-user"
jump_password = "env:SQIO_SSH_JUMP_PASSWORD"
jump_known_hosts = "/tmp/sqio_jump_known_hosts"
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
	if !conn.SSHTunnel.Enabled || conn.SSHTunnel.Password != "ssh-secret" || conn.SSHTunnel.KnownHosts != "/tmp/sqio_known_hosts" || conn.SSHTunnel.KeepAlive != "30s" ||
		!conn.SSHTunnel.Reconnect || conn.SSHTunnel.ReconnectAttempts != 3 || conn.SSHTunnel.JumpHost != "jump" || conn.SSHTunnel.JumpPort != 2022 ||
		conn.SSHTunnel.JumpUser != "jump-user" || conn.SSHTunnel.JumpPassword != "jump-secret" || conn.SSHTunnel.JumpKnownHosts != "/tmp/sqio_jump_known_hosts" {
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

func TestSaveUpsertAndRemoveConnection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := UpsertConnection(path, Connection{Name: "local", Driver: "sqlite", Database: "local.db", Readonly: true}); err != nil {
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
	if conn.Database != "local.db" || !conn.Readonly || cfg.Query.Timeout != "30s" {
		t.Fatalf("unexpected saved config: %+v", cfg)
	}
	if err := UpsertConnection(path, Connection{Name: "local", Driver: "sqlite", Database: "replaced.db"}); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	conn, err = cfg.Connection("local")
	if err != nil {
		t.Fatal(err)
	}
	if conn.Database != "replaced.db" || conn.Readonly {
		t.Fatalf("expected replaced connection, got %+v", conn)
	}
	if err := RemoveConnection(path, "local"); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.Connection("local"); err == nil {
		t.Fatal("expected removed connection")
	}
	if err := RemoveConnection(path, "missing"); err == nil {
		t.Fatal("expected missing remove error")
	}
}

func TestConfigFileErrors(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(badPath, []byte("[query\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(badPath); err == nil {
		t.Fatal("expected explicit config decode error")
	}
	if _, err := loadMutable(badPath); err == nil {
		t.Fatal("expected mutable config decode error")
	}
	filePath := filepath.Join(dir, "file")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Save(filepath.Join(filePath, "config.toml"), Default()); err == nil {
		t.Fatal("expected save parent directory error")
	}
	if err := mergeFile(&Config{}, filepath.Join(filePath, "config.toml")); err == nil {
		t.Fatal("expected merge file stat error")
	}
}

func TestDefaultPathFallback(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	path := DefaultPath()
	if !strings.HasSuffix(path, filepath.Join(".config", "sqio", "config.toml")) {
		t.Fatalf("unexpected fallback default path: %s", path)
	}
}

func TestMergeConfigPreservesZeroValueOverrides(t *testing.T) {
	base := Default()
	base.Formatter.Indent = 2
	base.Formatter.KeywordCase = "upper"
	base.Lint.Disable = []string{"select-star"}
	var override Config
	meta, err := toml.Decode(`
theme = "light"
editor = "vim"

[query]
timeout = "5s"
max_rows = 25
format = "json"

[formatter]
dialect = "postgres"
indent = 4
keyword_case = "lower"
identifier_case = "lower"
line_width = 120

[lint]
disable = ["implicit-join"]

[[connections]]
name = "local"
driver = "sqlite"
database = "local.db"
readonly = true
`, &override)
	if err != nil {
		t.Fatal(err)
	}
	merged := base
	mergeConfig(&merged, override, meta)
	if merged.Theme != "light" || merged.Editor != "vim" || merged.Query.Timeout != "5s" || merged.Query.MaxRows != 25 || merged.Query.Format != "json" {
		t.Fatalf("unexpected merged base fields: %+v", merged)
	}
	if merged.Formatter.Dialect != "postgres" || merged.Formatter.Indent != 4 || merged.Formatter.KeywordCase != "lower" || merged.Formatter.IdentifierCase != "lower" || merged.Formatter.LineWidth != 120 {
		t.Fatalf("unexpected merged formatter: %+v", merged.Formatter)
	}
	if len(merged.Lint.Disable) != 1 || merged.Lint.Disable[0] != "implicit-join" {
		t.Fatalf("unexpected merged linter: %+v", merged.Lint)
	}
	if len(merged.Connections) != 1 || merged.Connections[0].Name != "local" || !merged.Connections[0].Readonly {
		t.Fatalf("unexpected merged connections: %+v", merged.Connections)
	}
}
