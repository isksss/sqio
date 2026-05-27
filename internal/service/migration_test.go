package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationService(t *testing.T) {
	dir := t.TempDir()
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
	service := MigrationService{Driver: "sqlite", DSN: filepath.Join(dir, "test.db")}
	status, err := service.Status(context.Background(), migrationDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(status) != 1 || status[0].Applied {
		t.Fatalf("unexpected status: %+v", status)
	}
	result, err := service.Apply(context.Background(), migrationDir, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Applied) != 1 {
		t.Fatalf("unexpected apply result: %+v", result)
	}
	plan, err := service.Plan(context.Background(), migrationDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Rollback) != 1 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	rolledBack, err := service.Rollback(context.Background(), migrationDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rolledBack.Applied) != 1 {
		t.Fatalf("unexpected rollback result: %+v", rolledBack)
	}
	baselined, err := service.Baseline(context.Background(), migrationDir, "001")
	if err != nil {
		t.Fatal(err)
	}
	if len(baselined.Applied) != 1 {
		t.Fatalf("unexpected baseline result: %+v", baselined)
	}
}
