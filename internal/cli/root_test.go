package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
	"filippo.io/age/armor"
)

// TestMain verifies the behavior covered by this test helper or case.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "sqio-cli-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = os.Setenv("SQIO_HISTORY_PATH", filepath.Join(dir, "history.db"))
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// executeForTest verifies the behavior covered by this test helper or case.
func executeForTest(args ...string) (string, error) {
	cmd := newRootCommand()
	out := bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// TestExecSelectOneJSON verifies the behavior covered by this test helper or case.
func TestExecSelectOneJSON(t *testing.T) {
	out, err := executeForTest("exec", "--sql", "select 1", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"row_count": 1`) {
		t.Fatalf("expected row_count in output, got %s", out)
	}
}

// TestFmtSQL verifies the behavior covered by this test helper or case.
func TestFmtSQL(t *testing.T) {
	out, err := executeForTest("fmt", "--sql", "select id from users")
	if err != nil {
		t.Fatal(err)
	}
	if out != "SELECT id\n  FROM users\n" {
		t.Fatalf("unexpected output: %q", out)
	}
}

// TestQuerySQL verifies the behavior covered by this test helper or case.
func TestQuerySQL(t *testing.T) {
	out, err := executeForTest("query", "--sql", "select 1", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"row_count": 1`) {
		t.Fatalf("expected row_count in output, got %s", out)
	}
}

// TestQueryPick verifies the behavior covered by this test helper or case.
func TestQueryPick(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "query.sql"), []byte("select 1"), 0o644)
	if err != nil {
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
	out, err := executeForTest("query", "--pick", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"row_count": 1`) {
		t.Fatalf("expected row_count in output, got %s", out)
	}
}

// TestHistoryCommand verifies the behavior covered by this test helper or case.
func TestHistoryCommand(t *testing.T) {
	_, err := executeForTest("exec", "--sql", "select 1", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	out, err := executeForTest("history", "--format", "json", "--limit", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "select 1") {
		t.Fatalf("expected query in history, got %s", out)
	}
}

// TestLintIssue verifies the behavior covered by this test helper or case.
func TestLintIssue(t *testing.T) {
	out, err := executeForTest("lint", "--sql", "select * from users")
	if err == nil {
		t.Fatal("expected lint issue")
	}
	if ExitCode(err) != 1 {
		t.Fatalf("expected exit 1, got %d", ExitCode(err))
	}
	if !strings.Contains(out, "select-star") {
		t.Fatalf("expected select-star, got %s", out)
	}
}

// TestTables verifies the behavior covered by this test helper or case.
func TestTables(t *testing.T) {
	out, err := executeForTest("tables")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "users") {
		t.Fatalf("expected users table, got %s", out)
	}
}

// TestColumnsJSON verifies the behavior covered by this test helper or case.
func TestColumnsJSON(t *testing.T) {
	out, err := executeForTest("columns", "--table", "users", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"name":"id"`) {
		t.Fatalf("expected id column, got %s", out)
	}
}

// TestDDL verifies the behavior covered by this test helper or case.
func TestDDL(t *testing.T) {
	out, err := executeForTest("ddl", "--table", "users")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "CREATE TABLE users") {
		t.Fatalf("expected DDL, got %s", out)
	}
}

// TestSchemaExport verifies the behavior covered by this test helper or case.
func TestSchemaExport(t *testing.T) {
	out, err := executeForTest("schema", "export", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"tables"`) {
		t.Fatalf("expected schema json, got %s", out)
	}
}

// TestER verifies the behavior covered by this test helper or case.
func TestER(t *testing.T) {
	out, err := executeForTest("er", "--format", "mermaid")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "erDiagram") {
		t.Fatalf("expected mermaid ER, got %s", out)
	}
}

// TestInitCommandCreatesLocalConfig verifies the behavior covered by this test helper or case.
func TestInitCommandCreatesLocalConfig(t *testing.T) {
	dir := t.TempDir()
	current, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(current)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	out, err := executeForTest("init")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "sqio.toml" {
		t.Fatalf("unexpected output: %q", out)
	}
	content, err := os.ReadFile(filepath.Join(dir, "sqio.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `theme = "dark"`) || !strings.Contains(string(content), `[formatter]`) {
		t.Fatalf("default config was not written: %s", content)
	}
}

// TestInitCommandCreatesGlobalConfig verifies the behavior covered by this test helper or case.
func TestInitCommandCreatesGlobalConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	out, err := executeForTest("init", "-g")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config", "sqio", "config.toml")
	if strings.TrimSpace(out) != path {
		t.Fatalf("unexpected output: %q", out)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `max_rows = 1000`) || !strings.Contains(string(content), `disable = []`) {
		t.Fatalf("default config was not written: %s", content)
	}
}

// TestInitCommandRejectsExistingConfig verifies the behavior covered by this test helper or case.
func TestInitCommandRejectsExistingConfig(t *testing.T) {
	dir := t.TempDir()
	current, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(current)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sqio.toml"), []byte("theme = \"light\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = executeForTest("init")
	if err == nil {
		t.Fatal("expected init error")
	}
	if ExitCode(err) != ExitInternal {
		t.Fatalf("expected internal exit, got %d", ExitCode(err))
	}
	content, err := os.ReadFile(filepath.Join(dir, "sqio.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "theme = \"light\"\n" {
		t.Fatalf("existing config was overwritten: %s", content)
	}
}

// TestExecWithConfigConnection verifies the behavior covered by this test helper or case.
func TestExecWithConfigConnection(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	configPath := filepath.Join(dir, "config.toml")
	err := os.WriteFile(configPath, []byte(`[[connections]]
name = "local"
driver = "sqlite"
database = "`+dbPath+`"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	out, err := executeForTest("--config", configPath, "exec", "--conn", "local", "--sql", "select 1", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"row_count": 1`) {
		t.Fatalf("expected sqlite result, got %s", out)
	}
}

// TestMetadataWithConfigConnection verifies the behavior covered by this test helper or case.
func TestMetadataWithConfigConnection(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	configPath := filepath.Join(dir, "config.toml")
	err := os.WriteFile(configPath, []byte(`[[connections]]
name = "local"
driver = "sqlite"
database = "`+dbPath+`"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, err = executeForTest("--config", configPath, "exec", "--conn", "local", "--sql", "create table users (id integer primary key, name text)")
	if err != nil {
		t.Fatal(err)
	}
	out, err := executeForTest("--config", configPath, "tables", "--conn", "local")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "users" {
		t.Fatalf("expected users table, got %s", out)
	}
	out, err = executeForTest("--config", configPath, "columns", "--conn", "local", "--table", "users")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "id\tINTEGER") {
		t.Fatalf("expected sqlite columns, got %s", out)
	}
}

// TestReadonlyRejectsDangerousQuery verifies the behavior covered by this test helper or case.
func TestReadonlyRejectsDangerousQuery(t *testing.T) {
	_, err := executeForTest("exec", "--readonly", "--sql", "delete from users")
	if err == nil {
		t.Fatal("expected readonly error")
	}
	if ExitCode(err) != ExitSQLSyntax {
		t.Fatalf("expected sql syntax exit, got %d", ExitCode(err))
	}
}

// TestReadonlyRejectsMutatingQuery verifies the behavior covered by this test helper or case.
func TestReadonlyRejectsMutatingQuery(t *testing.T) {
	_, err := executeForTest("exec", "--readonly", "--sql", "insert into users (name) values ('a')")
	if err == nil {
		t.Fatal("expected readonly error")
	}
	if ExitCode(err) != ExitSQLSyntax {
		t.Fatalf("expected sql syntax exit, got %d", ExitCode(err))
	}
}

// TestResolveAgeEncryptedPassword verifies the behavior covered by this test helper or case.
func TestResolveAgeEncryptedPassword(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	var encrypted bytes.Buffer
	armored := armor.NewWriter(&encrypted)
	writer, err := age.Encrypt(armored, identity.Recipient())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("secret")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := armored.Close(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	identityPath := filepath.Join(dir, "identity.txt")
	if err := os.WriteFile(identityPath, []byte(identity.String()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(configPath, []byte(`[[connections]]
name = "local"
driver = "sqlite"
database = "`+filepath.Join(dir, "test.db")+`"
password = """`+encrypted.String()+`"""
password_encrypted = true
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	out, err := executeForTest("--config", configPath, "exec", "--conn", "local", "--age-identity", identityPath, "--sql", "select 1", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"row_count": 1`) {
		t.Fatalf("expected result, got %s", out)
	}
}

// TestSSHTunnelRejectsDSN verifies the behavior covered by this test helper or case.
func TestSSHTunnelRejectsDSN(t *testing.T) {
	_, err := executeForTest(
		"exec",
		"--driver", "postgres",
		"--dsn", "postgres://sqio:sqio@localhost:15432/sqio?sslmode=disable",
		"--ssh-tunnel",
		"--ssh-host", "bastion",
		"--ssh-user", "deploy",
		"--ssh-password", "secret",
		"--sql", "select 1",
	)
	if err == nil {
		t.Fatal("expected ssh tunnel dsn error")
	}
	if ExitCode(err) != ExitConnection {
		t.Fatalf("expected connection exit, got %d", ExitCode(err))
	}
}
