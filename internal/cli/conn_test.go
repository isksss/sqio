package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConnList(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	err := os.WriteFile(configPath, []byte(`[[connections]]
name = "local"
driver = "sqlite"
database = "`+filepath.Join(dir, "test.db")+`"
readonly = true
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	out, err := executeForTest("--config", configPath, "conn", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "local\tsqlite") || !strings.Contains(out, "readonly") {
		t.Fatalf("unexpected connection list: %s", out)
	}
	out, err = executeForTest("--config", configPath, "conn", "list", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"name": "local"`) || strings.Contains(out, "password") || strings.Contains(out, "dsn") {
		t.Fatalf("unexpected connection json: %s", out)
	}
}

func TestConnTestConfiguredSQLite(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	err := os.WriteFile(configPath, []byte(`[[connections]]
name = "local"
driver = "sqlite"
database = "`+filepath.Join(dir, "test.db")+`"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	out, err := executeForTest("--config", configPath, "conn", "test", "--conn", "local")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "ok\tlocal\tsqlite") {
		t.Fatalf("unexpected connection test output: %s", out)
	}
	out, err = executeForTest("--config", configPath, "conn", "test", "--conn", "local", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"connection":"local"`) || !strings.Contains(out, `"ok":true`) {
		t.Fatalf("unexpected connection test json: %s", out)
	}
}

func TestConnAddAndRemove(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	out, err := executeForTest("--config", configPath, "conn", "add", "local", "--driver", "sqlite", "--database", filepath.Join(dir, "test.db"), "--password", "env:SQIO_TEST_PASSWORD", "--readonly")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "SQIO_TEST_PASSWORD") || !strings.Contains(out, "saved\tlocal") {
		t.Fatalf("unexpected add output: %s", out)
	}
	out, err = executeForTest("--config", configPath, "conn", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "local\tsqlite") || strings.Contains(out, "SQIO_TEST_PASSWORD") {
		t.Fatalf("unexpected connection list after add: %s", out)
	}
	out, err = executeForTest("--config", configPath, "conn", "remove", "local")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "removed\tlocal") {
		t.Fatalf("unexpected remove output: %s", out)
	}
	out, err = executeForTest("--config", configPath, "conn", "list")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "local") {
		t.Fatalf("expected removed connection, got %s", out)
	}
}

func TestConnAddRejectsInvalidConnection(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	_, err := executeForTest("--config", configPath, "conn", "add", "bad", "--driver", "sqlite")
	if err == nil {
		t.Fatal("expected invalid connection error")
	}
	if ExitCode(err) != ExitInternal {
		t.Fatalf("expected internal exit, got %d", ExitCode(err))
	}
}

func TestConnTestErrors(t *testing.T) {
	_, err := executeForTest("conn", "test")
	if err == nil {
		t.Fatal("expected missing connection error")
	}
	if ExitCode(err) != ExitConnection {
		t.Fatalf("expected connection exit, got %d", ExitCode(err))
	}
	_, err = executeForTest("conn", "list", "--format", "yaml")
	if err == nil {
		t.Fatal("expected bad list format error")
	}
}

func TestConnLabels(t *testing.T) {
	if connTestLabel(connectionOptions{conn: "local"}) != "local" || connTestLabel(connectionOptions{}) != "direct" {
		t.Fatal("unexpected connection test label")
	}
	if readonlyLabel(true) != "readonly" || readonlyLabel(false) != "readwrite" {
		t.Fatal("unexpected readonly label")
	}
}
