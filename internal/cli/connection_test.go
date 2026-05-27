package cli

import (
	"strings"
	"testing"

	"github.com/isksss/sqio/internal/config"
)

func TestResolveConnectionHelpers(t *testing.T) {
	cfg := config.Config{Connections: []config.Connection{{Name: "local", Driver: "sqlite", Database: "test.db"}}}
	driver, dsn, err := resolveConnectionOptions(cfg, connectionOptions{conn: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if driver != "sqlite" || dsn != "test.db" {
		t.Fatalf("unexpected connection: %s %s", driver, dsn)
	}
	driver, dsn, err = resolveConnection(cfg, execOptions{connectionOptions: connectionOptions{driver: "sqlite", database: "direct.db"}})
	if err != nil {
		t.Fatal(err)
	}
	if driver != "sqlite" || dsn != "direct.db" {
		t.Fatalf("unexpected direct connection: %s %s", driver, dsn)
	}
	if _, _, err := resolveConnectionOptions(cfg, connectionOptions{conn: "missing"}); err == nil {
		t.Fatal("expected missing connection error")
	}
}

func TestConnectionSmallHelpers(t *testing.T) {
	if got := firstNonEmpty("", "a", "b"); got != "a" {
		t.Fatalf("unexpected first non-empty: %s", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Fatalf("unexpected empty fallback: %s", got)
	}
	if got := firstNonZero(0, 2); got != 2 {
		t.Fatalf("unexpected first non-zero: %d", got)
	}
	if got := firstNonZero(0, 0); got != 0 {
		t.Fatalf("unexpected zero fallback: %d", got)
	}
	for driver, want := range map[string]int{"postgres": 5432, "postgresql": 5432, "pgx": 5432, "mysql": 3306, "sqlserver": 1433, "oracle": 1521, "clickhouse": 9000, "sqlite": 0} {
		if got := defaultPort(driver); got != want {
			t.Fatalf("unexpected default port for %s: %d", driver, got)
		}
	}
}

func TestPrepareConnectionErrors(t *testing.T) {
	_, _, _, err := prepareConnection(t.Context(), config.Config{}, connectionOptions{driver: "sqlite"})
	if err == nil || !strings.Contains(err.Error(), "database path") {
		t.Fatalf("expected sqlite database path error, got %v", err)
	}
	_, _, _, err = prepareConnection(t.Context(), config.Config{}, connectionOptions{driver: "sqlite", database: "test.db", sshTunnel: true, sshHost: "bastion", sshUser: "deploy", sshPassword: "secret"})
	if err == nil || !strings.Contains(err.Error(), "remote port") {
		t.Fatalf("expected tunnel remote port error, got %v", err)
	}
	_, _, _, err = prepareConnection(t.Context(), config.Config{}, connectionOptions{driver: "sqlite", database: "test.db", sshKeepAlive: "bad"})
	if err == nil || !strings.Contains(err.Error(), "invalid duration") {
		t.Fatalf("expected keepalive duration error, got %v", err)
	}
}

func TestPrepareConnectionMergesAdvancedSSHOptions(t *testing.T) {
	cfg := config.Config{Connections: []config.Connection{{
		Name: "local", Driver: "postgres", Host: "db", Database: "app",
		SSHTunnel: config.SSHTunnel{
			Enabled: true, Host: "bastion", User: "deploy", Password: "secret",
			Reconnect: true, ReconnectAttempts: 2, JumpHost: "jump", JumpUser: "jump-user", JumpPassword: "jump-secret",
		},
	}}}
	_, _, _, err := prepareConnection(t.Context(), cfg, connectionOptions{conn: "local", sshTunnel: true, sshKeepAlive: "bad"})
	if err == nil || !strings.Contains(err.Error(), "invalid duration") {
		t.Fatalf("expected duration validation before tunnel start, got %v", err)
	}
}
