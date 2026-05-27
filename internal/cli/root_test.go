package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
	"filippo.io/age/armor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/isksss/sqio/internal/tui"
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

func TestExecuteEntrypoint(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"sqio", "--help"}
	if err := Execute(); err != nil {
		t.Fatal(err)
	}
	os.Args = []string{"sqio", "--quiet", "lint", "--sql", "select * from users"}
	if err := Execute(); err == nil {
		t.Fatal("expected lint issue from Execute")
	}
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

func TestFmtFileWriteAndCheck(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "query.sql")
	if err := os.WriteFile(path, []byte("select id from users"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := executeForTest("fmt", "--file", path, "--check"); err == nil {
		t.Fatal("expected fmt check error")
	}
	out, err := executeForTest("fmt", "--file", path, "--write")
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("expected no output for write, got %s", out)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "SELECT id\n  FROM users\n" {
		t.Fatalf("unexpected formatted file: %q", content)
	}
	if _, err := executeForTest("fmt", "--file", path, "--check"); err != nil {
		t.Fatal(err)
	}
	if _, err := executeForTest("fmt", "--sql", "select 1", "--write"); err == nil {
		t.Fatal("expected write without file error")
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

func TestHistoryRecordsExecutionDetails(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQIO_HISTORY_PATH", filepath.Join(dir, "history.db"))
	if _, err := executeForTest("exec", "--sql", "select 1", "--format", "json"); err != nil {
		t.Fatal(err)
	}
	if _, err := executeForTest("exec", "--sql", "select 2"); err == nil {
		t.Fatal("expected execution error")
	}
	out, err := executeForTest("history", "--format", "json", "--limit", "2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"success": false`) || !strings.Contains(out, `"error": "database connection is required for this SQL"`) {
		t.Fatalf("expected failed execution details, got %s", out)
	}
	if !strings.Contains(out, `"success": true`) || !strings.Contains(out, `"row_count": 1`) {
		t.Fatalf("expected successful execution details, got %s", out)
	}
	out, err = executeForTest("history", "--limit", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "error: database connection is required for this SQL") {
		t.Fatalf("expected table status, got %s", out)
	}
}

func TestHistoryFiltersAndMutations(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQIO_HISTORY_PATH", filepath.Join(dir, "history.db"))
	if _, err := executeForTest("exec", "--sql", "select 1", "--format", "json"); err != nil {
		t.Fatal(err)
	}
	if _, err := executeForTest("exec", "--sql", "select 1", "--format", "json"); err != nil {
		t.Fatal(err)
	}
	out, err := executeForTest("history", "--search", "select 1", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "select 1") {
		t.Fatalf("unexpected filtered history: %s", out)
	}
	out, err = executeForTest("history", "--limit", "1")
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		t.Fatalf("expected history row, got %q", out)
	}
	id := fields[0]
	if _, err := executeForTest("history", "favorite", id); err != nil {
		t.Fatal(err)
	}
	if _, err := executeForTest("history", "tag", id, "--tags", "smoke,report"); err != nil {
		t.Fatal(err)
	}
	out, err = executeForTest("history", "--favorite", "--tags", "report")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "favorite") || !strings.Contains(out, "smoke,report") {
		t.Fatalf("expected tagged favorite history, got %s", out)
	}
	if _, err := executeForTest("history", "unfavorite", id); err != nil {
		t.Fatal(err)
	}
	out, err = executeForTest("history", "run", id, "--format", "json", "--no-history")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"row_count": 1`) {
		t.Fatalf("expected rerun result, got %s", out)
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

func TestLintDialectFlag(t *testing.T) {
	out, err := executeForTest("lint", "--dialect", "postgres", "--sql", "select `id` from users limit 10,20", "--format", "json")
	if err == nil {
		t.Fatal("expected dialect lint issue")
	}
	if !strings.Contains(out, "postgres-backtick-identifier") || !strings.Contains(out, "postgres-limit-offset") {
		t.Fatalf("expected postgres dialect issues, got %s", out)
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

func TestSchemas(t *testing.T) {
	out, err := executeForTest("schemas")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "default") {
		t.Fatalf("expected fallback schema, got %s", out)
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

func TestSchemaDiffCommand(t *testing.T) {
	dir := t.TempDir()
	fromPath := filepath.Join(dir, "from.json")
	toPath := filepath.Join(dir, "to.json")
	if err := os.WriteFile(fromPath, []byte(`{"tables":[{"name":"users","columns":[{"name":"id","type":"integer"}]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(toPath, []byte(`{"tables":[{"name":"users","columns":[{"name":"id","type":"integer"},{"name":"email","type":"text"}]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := executeForTest("schema", "diff", "--from", fromPath, "--to", toPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "add_column\tusers\temail") {
		t.Fatalf("expected schema diff, got %s", out)
	}
	out, err = executeForTest("schema", "diff", "--from", fromPath, "--to", toPath, "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"type": "add_column"`) {
		t.Fatalf("expected schema diff json, got %s", out)
	}
	if _, err := executeForTest("schema", "diff", "--from", fromPath); err == nil {
		t.Fatal("expected missing to error")
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

func TestCompleteCommand(t *testing.T) {
	out, err := executeForTest("complete", "--prefix", "sel")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "SELECT\tkeyword") {
		t.Fatalf("expected SELECT completion, got %s", out)
	}
	out, err = executeForTest("complete", "--prefix", "na", "--table", "users", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"value":"name"`) || !strings.Contains(out, `"kind":"column"`) {
		t.Fatalf("expected column completion, got %s", out)
	}
	if _, err := executeForTest("complete", "--format", "yaml"); err == nil {
		t.Fatal("expected bad completion format error")
	}
}

func TestMetadataOutputFilesAndErrors(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "schema.json")
	out, err := executeForTest("schema", "export", "--format", "json", "--out", outPath)
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("expected no stdout for file output, got %s", out)
	}
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"tables"`) {
		t.Fatalf("expected schema file, got %s", content)
	}
	_, err = executeForTest("columns", "--table", "missing")
	if err == nil {
		t.Fatal("expected missing table error")
	}
	_, err = executeForTest("ddl", "--table", "missing")
	if err == nil {
		t.Fatal("expected missing ddl error")
	}
	_, err = executeForTest("er", "--format", "bad")
	if err == nil {
		t.Fatal("expected bad er format error")
	}
}

func TestExecOutputAndErrorBranches(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "result.json")
	out, err := executeForTest("exec", "--sql", "select 1", "--format", "json", "--out", outPath, "--no-history")
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("expected no stdout for file output, got %s", out)
	}
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"row_count": 1`) {
		t.Fatalf("unexpected output file: %s", content)
	}
	if _, err := executeForTest("exec", "--sql", "select 1", "--timeout", "bad"); err == nil {
		t.Fatal("expected bad timeout error")
	}
	if _, err := executeForTest("exec", "--sql", "select 1", "--format", "bad"); err == nil {
		t.Fatal("expected bad output format error")
	}
}

func TestExecAuditLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	if _, err := executeForTest("exec", "--sql", "select 1", "--format", "json", "--audit-log", path, "--no-history"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	log := string(content)
	if !strings.Contains(log, `"success":true`) || !strings.Contains(log, `"row_count":1`) || !strings.Contains(log, `"sql":"select 1"`) {
		t.Fatalf("unexpected audit log: %s", log)
	}
	if strings.Contains(log, "dsn") || strings.Contains(log, "password") {
		t.Fatalf("audit log should not include connection secrets: %s", log)
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

func TestConfigValidateCommand(t *testing.T) {
	dir := t.TempDir()
	validPath := filepath.Join(dir, "valid.toml")
	if err := os.WriteFile(validPath, []byte(`[[connections]]
name = "local"
driver = "sqlite"
database = "`+filepath.Join(dir, "test.db")+`"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := executeForTest("--config", validPath, "config", "validate")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "ok" {
		t.Fatalf("unexpected valid output: %s", out)
	}
	invalidPath := filepath.Join(dir, "invalid.toml")
	if err := os.WriteFile(invalidPath, []byte(`[query]
timeout = "bad"
format = "xml"

[[connections]]
name = "bad"
driver = "unsupported"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = executeForTest("--config", invalidPath, "config", "validate", "--format", "json")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(out, "query.timeout") || !strings.Contains(out, "connections") {
		t.Fatalf("unexpected invalid output: %s", out)
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

func TestDumpAndLoadCommands(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte(`[[connections]]
name = "local"
driver = "sqlite"
database = "`+dbPath+`"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := executeForTest("--config", configPath, "exec", "--conn", "local", "--sql", "create table users (id integer primary key, name text)"); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(dir, "users.csv")
	if err := os.WriteFile(inputPath, []byte("id,name\n1,alice\n2,bob\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := executeForTest("--config", configPath, "load", "--conn", "local", "--table", "users", "--file", inputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "OK (2 rows)") {
		t.Fatalf("unexpected load output: %s", out)
	}
	out, err = executeForTest("--config", configPath, "dump", "--conn", "local", "--table", "users", "--format", "csv", "--max-rows", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "id,name\n1,alice\n") || strings.Contains(out, "bob") {
		t.Fatalf("unexpected dump output: %s", out)
	}
	if _, err := executeForTest("--config", configPath, "dump", "--conn", "local"); err == nil {
		t.Fatal("expected missing dump table error")
	}
	if _, err := executeForTest("--config", configPath, "load", "--conn", "local", "--table", "users"); err == nil {
		t.Fatal("expected missing load file error")
	}
}

func TestLoadJSONCommand(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte(`[[connections]]
name = "local"
driver = "sqlite"
database = "`+dbPath+`"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := executeForTest("--config", configPath, "exec", "--conn", "local", "--sql", "create table users (id integer primary key, name text)"); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(dir, "users.json")
	if err := os.WriteFile(inputPath, []byte(`[{"id":1,"name":"alice"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := executeForTest("--config", configPath, "load", "--conn", "local", "--table", "users", "--file", inputPath, "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "OK (1 rows)") {
		t.Fatalf("unexpected json load output: %s", out)
	}
	inputPath = filepath.Join(dir, "users.jsonl")
	if err := os.WriteFile(inputPath, []byte("{\"id\":2,\"name\":\"bob\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = executeForTest("--config", configPath, "load", "--conn", "local", "--table", "users", "--file", inputPath, "--format", "jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "OK (1 rows)") {
		t.Fatalf("unexpected jsonl load output: %s", out)
	}
	out, err = executeForTest("--config", configPath, "dump", "--conn", "local", "--table", "users", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "alice") || !strings.Contains(out, "bob") {
		t.Fatalf("expected loaded json rows, got %s", out)
	}
	inputPath = filepath.Join(dir, "users.yaml")
	if err := os.WriteFile(inputPath, []byte("- id: 3\n  name: carol\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = executeForTest("--config", configPath, "load", "--conn", "local", "--table", "users", "--file", inputPath, "--format", "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "OK (1 rows)") {
		t.Fatalf("unexpected yaml load output: %s", out)
	}
}

func TestDumpAndLoadGzipCommands(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte(`[[connections]]
name = "local"
driver = "sqlite"
database = "`+dbPath+`"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := executeForTest("--config", configPath, "exec", "--conn", "local", "--sql", "create table users (id integer primary key, name text); insert into users (id, name) values (1, 'alice')"); err != nil {
		t.Fatal(err)
	}
	dumpPath := filepath.Join(dir, "users.csv.gz")
	if _, err := executeForTest("--config", configPath, "dump", "--conn", "local", "--table", "users", "--format", "csv", "--out", dumpPath); err != nil {
		t.Fatal(err)
	}
	if _, err := executeForTest("--config", configPath, "exec", "--conn", "local", "--sql", "delete from users where id = 1"); err != nil {
		t.Fatal(err)
	}
	out, err := executeForTest("--config", configPath, "load", "--conn", "local", "--table", "users", "--file", dumpPath, "--format", "csv")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "OK (1 rows)") {
		t.Fatalf("unexpected gzip load output: %s", out)
	}
	out, err = executeForTest("--config", configPath, "dump", "--conn", "local", "--table", "users", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "alice") {
		t.Fatalf("expected restored row, got %s", out)
	}
	plainPath := filepath.Join(dir, "plain.csv")
	if err := os.WriteFile(plainPath, []byte("id,name\n3,carol\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := executeForTest("--config", configPath, "load", "--conn", "local", "--table", "users", "--file", plainPath, "--gzip"); err == nil {
		t.Fatal("expected invalid gzip input error")
	}
}

func TestEditCommands(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte(`[[connections]]
name = "local"
driver = "sqlite"
database = "`+dbPath+`"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := executeForTest("--config", configPath, "exec", "--conn", "local", "--sql", "create table users (id integer primary key, name text, status text)"); err != nil {
		t.Fatal(err)
	}
	out, err := executeForTest("--config", configPath, "edit", "insert", "--conn", "local", "--table", "users", "--set", "name=alice", "--set", "status=new")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "OK (1 rows)") {
		t.Fatalf("unexpected insert output: %s", out)
	}
	out, err = executeForTest("--config", configPath, "edit", "update", "--conn", "local", "--table", "users", "--set", "status=active", "--where", "name = 'alice'")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "OK (1 rows)") {
		t.Fatalf("unexpected update output: %s", out)
	}
	out, err = executeForTest("--config", configPath, "exec", "--conn", "local", "--sql", "select status from users where name = 'alice'", "--format", "json", "--no-history")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "active") {
		t.Fatalf("expected updated row, got %s", out)
	}
	out, err = executeForTest("--config", configPath, "edit", "delete", "--conn", "local", "--table", "users", "--where", "name = 'alice'")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "OK (1 rows)") {
		t.Fatalf("unexpected delete output: %s", out)
	}
	if _, err := executeForTest("--config", configPath, "edit", "update", "--conn", "local", "--table", "users", "--set", "status=bad"); err == nil {
		t.Fatal("expected missing where error")
	}
}

func TestExplainCommandWithConfigConnection(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte(`[[connections]]
name = "local"
driver = "sqlite"
database = "`+dbPath+`"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := executeForTest("--config", configPath, "exec", "--conn", "local", "--sql", "create table users (id integer primary key, name text)"); err != nil {
		t.Fatal(err)
	}
	out, err := executeForTest("--config", configPath, "explain", "--conn", "local", "--sql", "select id from users", "--format", "json", "--no-history")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"columns"`) || !strings.Contains(out, "users") {
		t.Fatalf("expected explain plan output, got %s", out)
	}
}

func TestAccessCommandsWithSQLite(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte(`[[connections]]
name = "local"
driver = "sqlite"
database = "`+filepath.Join(dir, "test.db")+`"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := executeForTest("--config", configPath, "roles", "--conn", "local", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("expected empty sqlite roles, got %s", out)
	}
	out, err = executeForTest("--config", configPath, "grants", "--conn", "local", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("expected empty sqlite grants, got %s", out)
	}
	out, err = executeForTest("--config", configPath, "roles", "--conn", "local")
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("expected empty sqlite roles table, got %s", out)
	}
	out, err = executeForTest("--config", configPath, "grants", "--conn", "local")
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("expected empty sqlite grants table, got %s", out)
	}
	if _, err := executeForTest("roles"); err == nil {
		t.Fatal("expected missing connection error")
	}
	if _, err := executeForTest("--config", configPath, "roles", "--conn", "local", "--format", "yaml"); err == nil {
		t.Fatal("expected bad roles format error")
	}
	if _, err := executeForTest("--config", configPath, "grants", "--conn", "local", "--format", "yaml"); err == nil {
		t.Fatal("expected bad grants format error")
	}
}

func TestMigrateCommands(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	configPath := filepath.Join(dir, "config.toml")
	migrationDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(migrationDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`[[connections]]
name = "local"
driver = "sqlite"
database = "`+dbPath+`"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationDir, "001_create_users.sql"), []byte("create table users (id integer primary key, name text);"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationDir, "001_create_users.down.sql"), []byte("drop table users;"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationDir, "002_insert_users.sql"), []byte("insert into users (name) values ('alice');"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationDir, "002_insert_users.down.sql"), []byte("delete from users where name = 'alice';"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := executeForTest("--config", configPath, "migrate", "status", "--conn", "local", "--dir", migrationDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "001\tpending") || !strings.Contains(out, "002\tpending") {
		t.Fatalf("unexpected migration status: %s", out)
	}
	out, err = executeForTest("--config", configPath, "migrate", "apply", "--conn", "local", "--dir", migrationDir, "--limit", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "001\tapplied") || strings.Contains(out, "002\tapplied") {
		t.Fatalf("unexpected migration apply output: %s", out)
	}
	out, err = executeForTest("--config", configPath, "migrate", "status", "--conn", "local", "--dir", migrationDir, "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"version": "001"`) || !strings.Contains(out, `"applied": true`) {
		t.Fatalf("unexpected migration status json: %s", out)
	}
	out, err = executeForTest("--config", configPath, "migrate", "apply", "--conn", "local", "--dir", migrationDir, "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"version": "002"`) {
		t.Fatalf("unexpected migration apply json: %s", out)
	}
	out, err = executeForTest("--config", configPath, "migrate", "plan", "--conn", "local", "--dir", migrationDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "rollback\t002\t002_insert_users") {
		t.Fatalf("unexpected migration plan: %s", out)
	}
	out, err = executeForTest("--config", configPath, "migrate", "plan", "--conn", "local", "--dir", migrationDir, "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"rollback"`) || !strings.Contains(out, `"version": "002"`) {
		t.Fatalf("unexpected migration plan json: %s", out)
	}
	out, err = executeForTest("--config", configPath, "migrate", "rollback", "--conn", "local", "--dir", migrationDir, "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"version": "002"`) {
		t.Fatalf("unexpected rollback json output: %s", out)
	}
	if _, err := executeForTest("--config", configPath, "migrate", "apply", "--conn", "local", "--dir", migrationDir); err != nil {
		t.Fatal(err)
	}
	out, err = executeForTest("--config", configPath, "migrate", "rollback", "--conn", "local", "--dir", migrationDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "002\trolled-back") {
		t.Fatalf("unexpected rollback output: %s", out)
	}
	out, err = executeForTest("--config", configPath, "migrate", "baseline", "--conn", "local", "--dir", migrationDir, "--version", "002", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"version": "002"`) {
		t.Fatalf("unexpected baseline json output: %s", out)
	}
	out, err = executeForTest("--config", configPath, "migrate", "baseline", "--conn", "local", "--dir", migrationDir, "--version", "002", "--quiet")
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("expected quiet up-to-date baseline output, got %s", out)
	}
	if _, err := executeForTest("--config", configPath, "migrate", "status", "--conn", "local", "--dir", migrationDir, "--format", "yaml"); err == nil {
		t.Fatal("expected bad migration format error")
	}
	if _, err := executeForTest("--config", configPath, "migrate", "plan", "--conn", "local", "--dir", migrationDir, "--format", "yaml"); err == nil {
		t.Fatal("expected bad migration plan format error")
	}
	if _, err := executeForTest("--config", configPath, "migrate", "rollback", "--conn", "local", "--dir", migrationDir, "--format", "yaml"); err == nil {
		t.Fatal("expected bad migration rollback format error")
	}
	if _, err := executeForTest("--config", configPath, "migrate", "apply", "--conn", "local", "--dir", migrationDir, "--format", "yaml"); err == nil {
		t.Fatal("expected bad migration apply format error")
	}
	if _, err := executeForTest("--config", configPath, "migrate", "baseline", "--conn", "local", "--dir", migrationDir, "--version", "002", "--format", "yaml"); err == nil {
		t.Fatal("expected bad migration baseline format error")
	}
	if _, err := executeForTest("migrate", "status"); err == nil {
		t.Fatal("expected missing connection error")
	}
	if appliedLabel(false) != "pending" {
		t.Fatal("unexpected pending migration label")
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
	_, err = executeForTest("--config", configPath, "exec", "--conn", "local", "--sql", "create table users (id integer primary key, name text); create unique index users_name_idx on users(name)")
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
	out, err = executeForTest("--config", configPath, "tables", "--conn", "local", "--format", "json", "--schema", "main")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"name":"users"`) {
		t.Fatalf("expected users table json, got %s", out)
	}
	out, err = executeForTest("--config", configPath, "schemas", "--conn", "local")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "main") {
		t.Fatalf("expected sqlite main schema, got %s", out)
	}
	out, err = executeForTest("--config", configPath, "schemas", "--conn", "local", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"main"`) {
		t.Fatalf("expected sqlite schema json, got %s", out)
	}
	out, err = executeForTest("--config", configPath, "columns", "--conn", "local", "--table", "users")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "id\tINTEGER") {
		t.Fatalf("expected sqlite columns, got %s", out)
	}
	out, err = executeForTest("--config", configPath, "indexes", "--conn", "local", "--table", "users")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "users_name_idx\tname\tunique") {
		t.Fatalf("expected sqlite indexes, got %s", out)
	}
	out, err = executeForTest("--config", configPath, "indexes", "--conn", "local", "--table", "users", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"name":"users_name_idx"`) {
		t.Fatalf("expected sqlite indexes json, got %s", out)
	}
	ddlPath := filepath.Join(dir, "users.sql")
	out, err = executeForTest("--config", configPath, "ddl", "--conn", "local", "--table", "users", "--out", ddlPath)
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("expected no DDL stdout for file output, got %s", out)
	}
	ddlContent, err := os.ReadFile(ddlPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ddlContent), "CREATE TABLE users") {
		t.Fatalf("expected DDL file, got %s", ddlContent)
	}
	out, err = executeForTest("--config", configPath, "schema", "export", "--conn", "local", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"indexes"`) || !strings.Contains(out, "users_name_idx") {
		t.Fatalf("expected schema indexes, got %s", out)
	}
	erPath := filepath.Join(dir, "er.mmd")
	out, err = executeForTest("--config", configPath, "er", "--conn", "local", "--out", erPath)
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("expected no ER stdout for file output, got %s", out)
	}
	erContent, err := os.ReadFile(erPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(erContent), "erDiagram") {
		t.Fatalf("expected ER file, got %s", erContent)
	}
	badOut := filepath.Join(dir, "missing", "out.txt")
	if _, err := executeForTest("--config", configPath, "tables", "--conn", "local", "--out", badOut); err == nil {
		t.Fatal("expected metadata output path error")
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

// TestPlainPasswordWithAgeIdentityIsNotDecrypted verifies the behavior covered by this test helper or case.
func TestPlainPasswordWithAgeIdentityIsNotDecrypted(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	err := os.WriteFile(configPath, []byte(`[[connections]]
name = "local"
driver = "sqlite"
database = "`+filepath.Join(dir, "test.db")+`"
password = "plain"
password_encrypted = false
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	identityPath := filepath.Join(dir, "identity.txt")
	if err := os.WriteFile(identityPath, []byte("not used\n"), 0o600); err != nil {
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

// TestEncryptedPasswordRequiresAgeIdentity verifies the behavior covered by this test helper or case.
func TestEncryptedPasswordRequiresAgeIdentity(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	err := os.WriteFile(configPath, []byte(`[[connections]]
name = "local"
driver = "sqlite"
database = "`+filepath.Join(dir, "test.db")+`"
password = "encrypted"
password_encrypted = true
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, err = executeForTest("--config", configPath, "exec", "--conn", "local", "--sql", "select 1")
	if err == nil {
		t.Fatal("expected age identity error")
	}
	if ExitCode(err) != ExitConnection {
		t.Fatalf("expected connection exit, got %d", ExitCode(err))
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

func TestTUICommandUsesConfiguredConnection(t *testing.T) {
	oldProgram := newTUIProgram
	t.Cleanup(func() { newTUIProgram = oldProgram })
	var ran bool
	newTUIProgram = func(model tea.Model) tuiProgram {
		tuiModel, ok := model.(tui.Model)
		if !ok {
			t.Fatalf("unexpected TUI model type: %T", model)
		}
		if tuiModel.Init() == nil {
			t.Fatal("expected TUI init command")
		}
		return fakeTUIProgram{run: func() (tea.Model, error) {
			ran = true
			return model, nil
		}}
	}
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte(`[[connections]]
name = "local"
driver = "sqlite"
database = "`+filepath.Join(dir, "test.db")+`"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NO_COLOR", "1")
	if _, err := executeForTest("--config", configPath, "tui", "--conn", "local"); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("expected TUI program to run")
	}
}

func TestTUICommandReportsProgramError(t *testing.T) {
	oldProgram := newTUIProgram
	t.Cleanup(func() { newTUIProgram = oldProgram })
	newTUIProgram = func(model tea.Model) tuiProgram {
		return fakeTUIProgram{run: func() (tea.Model, error) {
			return model, errors.New("boom")
		}}
	}
	_, err := executeForTest("tui")
	if err == nil {
		t.Fatal("expected TUI program error")
	}
	if ExitCode(err) != ExitInternal {
		t.Fatalf("expected internal exit, got %d", ExitCode(err))
	}
}

type fakeTUIProgram struct {
	run func() (tea.Model, error)
}

func (p fakeTUIProgram) Run() (tea.Model, error) {
	return p.run()
}
