package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestAccessSQLite(t *testing.T) {
	cfg := Config{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "test.db")}
	roles, err := Roles(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 0 {
		t.Fatalf("expected empty sqlite roles, got %+v", roles)
	}
	grants, err := Grants(context.Background(), cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 0 {
		t.Fatalf("expected empty sqlite grants, got %+v", grants)
	}
}

func TestSplitMySQLAccount(t *testing.T) {
	name, host := splitMySQLAccount("'alice'@'localhost'")
	if name != "alice" || host != "localhost" {
		t.Fatalf("unexpected mysql account split: %s %s", name, host)
	}
	name, host = splitMySQLAccount("root")
	if name != "root" || host != "" {
		t.Fatalf("unexpected mysql account without host: %s %s", name, host)
	}
}

func TestPostgresAccessWithTestDriver(t *testing.T) {
	conn, err := sql.Open("sqio_meta_test", "postgres")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	roles, err := postgresRoles(context.Background(), conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 2 || roles[1].Name != "admin" || !roles[1].Superuser || !roles[1].CreateRole || !roles[1].CreateDB {
		t.Fatalf("unexpected postgres roles: %+v", roles)
	}
	grants, err := postgresGrants(context.Background(), conn, "app_user")
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 1 || grants[0].Grantee != "app_user" || grants[0].Object != "public.users" || grants[0].Privilege != "SELECT" || !grants[0].Grantable {
		t.Fatalf("unexpected postgres grants: %+v", grants)
	}
}

func TestMySQLAccessWithTestDriver(t *testing.T) {
	conn, err := sql.Open("sqio_meta_test", "mysql")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	roles, err := mysqlRoles(context.Background(), conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || roles[0].Name != "app" || roles[0].Host != "localhost" || !roles[0].Login {
		t.Fatalf("unexpected mysql roles: %+v", roles)
	}
	grants, err := mysqlGrants(context.Background(), conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 2 || grants[0].Raw == "" {
		t.Fatalf("unexpected mysql grants: %+v", grants)
	}
}

func TestAccessDispatchWithTestDriver(t *testing.T) {
	oldOpen := openConnection
	t.Cleanup(func() { openConnection = oldOpen })
	openConnection = func(_ context.Context, cfg Config) (*sql.DB, string, error) {
		driverName := cfg.Driver
		if driverName == "pgx" {
			driverName = "postgres"
		}
		conn, err := sql.Open("sqio_meta_test", driverName)
		return conn, cfg.Driver, err
	}
	roles, err := Roles(context.Background(), Config{Driver: "pgx"})
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 2 {
		t.Fatalf("unexpected postgres roles: %+v", roles)
	}
	grants, err := Grants(context.Background(), Config{Driver: "pgx"}, "app_user")
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 1 || grants[0].Grantee != "app_user" {
		t.Fatalf("unexpected postgres grants: %+v", grants)
	}
	roles, err = Roles(context.Background(), Config{Driver: "mysql"})
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || roles[0].Name != "app" {
		t.Fatalf("unexpected mysql roles: %+v", roles)
	}
	if _, err := Grants(context.Background(), Config{Driver: "mysql"}, "app"); err == nil {
		t.Fatal("expected mysql role-filter error")
	}
	roles, err = Roles(context.Background(), Config{Driver: "sqlite"})
	if err != nil || len(roles) != 0 {
		t.Fatalf("unexpected sqlite roles: %+v err=%v", roles, err)
	}
	if _, err := Roles(context.Background(), Config{Driver: "unsupported"}); err == nil {
		t.Fatal("expected unsupported roles driver error")
	}
	if _, err := Grants(context.Background(), Config{Driver: "unsupported"}, ""); err == nil {
		t.Fatal("expected unsupported grants driver error")
	}
}
