package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationStatusAndApply(t *testing.T) {
	dir := t.TempDir()
	migrationDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(migrationDir, 0o755); err != nil {
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
	if err := os.WriteFile(filepath.Join(migrationDir, "README.md"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Driver: "sqlite", DSN: filepath.Join(dir, "test.db")}
	status, err := MigrationStatus(context.Background(), cfg, migrationDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(status) != 2 || status[0].Version != "001" || status[0].Applied {
		t.Fatalf("unexpected initial status: %+v", status)
	}
	if status[0].Checksum == "" {
		t.Fatal("expected migration checksum")
	}
	result, err := ApplyMigrations(context.Background(), cfg, migrationDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Applied) != 1 || result.Applied[0].Version != "001" {
		t.Fatalf("unexpected first apply result: %+v", result)
	}
	status, err = MigrationStatus(context.Background(), cfg, migrationDir)
	if err != nil {
		t.Fatal(err)
	}
	if !status[0].Applied || status[1].Applied {
		t.Fatalf("unexpected partial status: %+v", status)
	}
	result, err = ApplyMigrations(context.Background(), cfg, migrationDir, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Applied) != 1 || result.Applied[0].Version != "002" {
		t.Fatalf("unexpected second apply result: %+v", result)
	}
	plan, err := PlanMigrations(context.Background(), cfg, migrationDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Pending) != 0 || len(plan.Rollback) != 1 || plan.Rollback[0].Version != "002" {
		t.Fatalf("unexpected migration plan: %+v", plan)
	}
	queryResult, err := Execute(context.Background(), cfg, "select name from users", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if queryResult.RowCount != 1 || queryResult.Rows[0][0] != "alice" {
		t.Fatalf("unexpected migrated data: %+v", queryResult)
	}
	rollback, err := RollbackMigrations(context.Background(), cfg, migrationDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rollback.Applied) != 1 || rollback.Applied[0].Version != "002" {
		t.Fatalf("unexpected rollback result: %+v", rollback)
	}
	queryResult, err = Execute(context.Background(), cfg, "select name from users", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if queryResult.RowCount != 0 {
		t.Fatalf("expected rolled back data, got %+v", queryResult)
	}
}

func TestMigrationErrorsAndVersion(t *testing.T) {
	if _, err := readMigrations(""); err == nil {
		t.Fatal("expected empty dir error")
	}
	if migrationVersion("20260101-create-users.sql") != "20260101" {
		t.Fatal("unexpected dash migration version")
	}
	if migrationVersion("baseline.sql") != "baseline" {
		t.Fatal("unexpected plain migration version")
	}
	if migrationName("001_create_users.up.sql") != "001_create_users" || migrationName("001_create_users.down.sql") != "001_create_users" {
		t.Fatal("unexpected migration name")
	}
}

func TestRollbackRequiresDownMigration(t *testing.T) {
	dir := t.TempDir()
	migrationDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(migrationDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationDir, "001_create_users.sql"), []byte("create table users (id integer primary key);"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Driver: "sqlite", DSN: filepath.Join(dir, "test.db")}
	if _, err := ApplyMigrations(context.Background(), cfg, migrationDir, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := RollbackMigrations(context.Background(), cfg, migrationDir, 1); err == nil {
		t.Fatal("expected missing down migration error")
	}
}

func TestMigrationApplyFailureRecordsDirtyState(t *testing.T) {
	dir := t.TempDir()
	migrationDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(migrationDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationDir, "001_bad.sql"), []byte("insert into missing_table values (1);"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Driver: "sqlite", DSN: filepath.Join(dir, "test.db")}
	if _, err := ApplyMigrations(context.Background(), cfg, migrationDir, 0); err == nil {
		t.Fatal("expected migration apply error")
	}
	status, err := MigrationStatus(context.Background(), cfg, migrationDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(status) != 1 || !status[0].Applied || !status[0].Dirty {
		t.Fatalf("failed migration should be recorded as dirty: %+v", status)
	}
	if _, err := ApplyMigrations(context.Background(), cfg, migrationDir, 0); err == nil {
		t.Fatal("expected dirty migration error")
	}
}

func TestMigrationChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	migrationDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(migrationDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(migrationDir, "001_create_users.sql")
	if err := os.WriteFile(path, []byte("create table users (id integer primary key);"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Driver: "sqlite", DSN: filepath.Join(dir, "test.db")}
	if _, err := ApplyMigrations(context.Background(), cfg, migrationDir, 0); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("create table users (id integer primary key, name text);"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanMigrations(context.Background(), cfg, migrationDir, 1); err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestBaselineMigrations(t *testing.T) {
	dir := t.TempDir()
	migrationDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(migrationDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationDir, "001_create_users.sql"), []byte("create table users (id integer primary key);"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationDir, "002_insert_users.sql"), []byte("insert into users (id) values (1);"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Driver: "sqlite", DSN: filepath.Join(dir, "test.db")}
	if _, err := BaselineMigrations(context.Background(), cfg, migrationDir, ""); err == nil {
		t.Fatal("expected missing baseline version error")
	}
	result, err := BaselineMigrations(context.Background(), cfg, migrationDir, "001")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Applied) != 1 || result.Applied[0].Version != "001" {
		t.Fatalf("unexpected baseline result: %+v", result)
	}
	plan, err := PlanMigrations(context.Background(), cfg, migrationDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Pending) != 1 || plan.Pending[0].Version != "002" {
		t.Fatalf("unexpected plan after baseline: %+v", plan)
	}
	result, err = BaselineMigrations(context.Background(), cfg, migrationDir, "001")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Applied) != 0 {
		t.Fatalf("expected no additional baseline rows, got %+v", result)
	}
}

func TestMigrationHelpersAndLegacyColumns(t *testing.T) {
	dir := t.TempDir()
	migrationDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(filepath.Join(migrationDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationDir, "002_second.sql"), []byte("select 2;"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationDir, "001_first.sql"), []byte("select 1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationDir, "001_first.down.sql"), []byte("select -1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	migrations, err := readMigrations(migrationDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 2 || migrations[0].Version != "001" || migrations[0].DownPath == "" || migrations[1].Version != "002" {
		t.Fatalf("unexpected migrations: %+v", migrations)
	}
	if _, err := migrationChecksum(filepath.Join(migrationDir, "missing.sql")); err == nil {
		t.Fatal("expected missing checksum file error")
	}

	path := filepath.Join(dir, "legacy.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`create table sqio_migrations (
version text primary key,
name text not null,
applied_at text not null
)`); err != nil {
		t.Fatal(err)
	}
	if err := ensureMigrationColumns(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	columns, err := migrationColumns(context.Background(), conn)
	if err != nil {
		t.Fatal(err)
	}
	if !columns["checksum"] || !columns["dirty"] {
		t.Fatalf("expected migrated columns, got %+v", columns)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationsWithPlaceholderDriver(t *testing.T) {
	oldOpen := openConnection
	t.Cleanup(func() { openConnection = oldOpen })
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	openConnection = func(ctx context.Context, cfg Config) (*sql.DB, string, error) {
		conn, _, err := Open(ctx, Config{Driver: "sqlite", DSN: dbPath})
		return conn, "pgx", err
	}
	migrationDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(migrationDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationDir, "001_create_users.sql"), []byte("create table users (id integer primary key);"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationDir, "001_create_users.down.sql"), []byte("drop table users;"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Driver: "pgx"}
	if _, err := ApplyMigrations(context.Background(), cfg, migrationDir, 0); err != nil {
		t.Fatal(err)
	}
	status, err := MigrationStatus(context.Background(), cfg, migrationDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(status) != 1 || !status[0].Applied {
		t.Fatalf("unexpected placeholder driver status: %+v", status)
	}
	if _, err := RollbackMigrations(context.Background(), cfg, migrationDir, 0); err != nil {
		t.Fatal(err)
	}
}
