package db

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// TestExecuteSQLite verifies the behavior covered by this test helper or case.
func TestExecuteSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	_, err := Execute(context.Background(), Config{Driver: "sqlite", DSN: path}, `
create table users (id integer primary key, name text);
insert into users (name) values ('alice');
select id, name from users;
`, ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(context.Background(), Config{Driver: "sqlite", DSN: path}, `select id, name from users;`, ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 1 {
		t.Fatalf("expected 1 row, got %d", result.RowCount)
	}
	if result.Rows[0][1] != "alice" {
		t.Fatalf("expected alice, got %#v", result.Rows[0][1])
	}
}

// TestExecuteReturnsExecError verifies the behavior covered by this test helper or case.
func TestExecuteReturnsExecError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	_, err := Execute(context.Background(), Config{Driver: "sqlite", DSN: path}, `insert into missing_table values (1);`, ExecuteOptions{})
	if err == nil {
		t.Fatal("expected exec error")
	}
	if !strings.Contains(err.Error(), "missing_table") {
		t.Fatalf("expected missing table error, got %v", err)
	}
}

// TestMetadataSQLite verifies the behavior covered by this test helper or case.
func TestMetadataSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	_, err := Execute(context.Background(), Config{Driver: "sqlite", DSN: path}, `
create table roles (id integer primary key, name text not null unique);
create table users (id integer primary key, name text not null default 'anonymous', email text unique, active boolean default 1, created_at datetime default CURRENT_TIMESTAMP, role_id integer references roles(id));
`, ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	schema, err := Metadata(context.Background(), Config{Driver: "sqlite", DSN: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(schema.Tables))
	}
	users := schema.Tables[1]
	if users.Name != "users" {
		t.Fatalf("expected users table, got %s", users.Name)
	}
	if len(users.Columns) != 6 {
		t.Fatalf("expected 6 columns, got %d", len(users.Columns))
	}
	types := map[string]string{}
	defaults := map[string]string{}
	unique := map[string]bool{}
	references := map[string]string{}
	for _, column := range users.Columns {
		types[column.Name] = strings.ToLower(column.Type)
		defaults[column.Name] = column.Default
		unique[column.Name] = column.Unique
		references[column.Name] = column.References
	}
	for name, want := range map[string]string{
		"id":         "integer",
		"name":       "text",
		"email":      "text",
		"active":     "boolean",
		"created_at": "datetime",
		"role_id":    "integer",
	} {
		if types[name] != want {
			t.Fatalf("expected %s type %s, got %s", name, want, types[name])
		}
	}
	for name, want := range map[string]string{
		"name":       "'anonymous'",
		"active":     "1",
		"created_at": "CURRENT_TIMESTAMP",
	} {
		if defaults[name] != want {
			t.Fatalf("expected %s default %s, got %s", name, want, defaults[name])
		}
	}
	if !unique["email"] {
		t.Fatal("expected email to be unique")
	}
	if references["role_id"] != `"roles"("id")` {
		t.Fatalf("expected role_id reference, got %s", references["role_id"])
	}
}

// TestDDLBuildersPreserveDialectTypes verifies the behavior covered by this test helper or case.
func TestDDLBuildersPreserveDialectTypes(t *testing.T) {
	table := TableInfo{
		Name: "events",
		Columns: []ColumnInfo{
			{Name: "id", Type: "bigint", Primary: true},
			{Name: "name", Type: "character varying(255)", Nullable: false, Default: "'anonymous'::character varying"},
			{Name: "amount", Type: "numeric(10,2)", Nullable: true, Unique: true},
			{Name: "created_at", Type: "timestamp with time zone", Nullable: false, Default: "now()"},
			{Name: "user_id", Type: "bigint", Nullable: false, References: `"public"."users"("id")`},
		},
	}
	postgres := postgresDDL(table)
	for _, want := range []string{"character varying(255)", "numeric(10,2)", "timestamp with time zone", "DEFAULT 'anonymous'::character varying", "DEFAULT now()", "UNIQUE", `REFERENCES "public"."users"("id")`} {
		if !strings.Contains(postgres, want) {
			t.Fatalf("expected postgres ddl to contain %q, got %s", want, postgres)
		}
	}
	mysql := mysqlDDL(TableInfo{
		Name: "events",
		Columns: []ColumnInfo{
			{Name: "id", Type: "bigint unsigned", Primary: true},
			{Name: "name", Type: "varchar(255)", Nullable: false, Default: "'anonymous'"},
			{Name: "payload", Type: "json", Nullable: true, Unique: true},
			{Name: "created_at", Type: "datetime(6)", Nullable: false, Default: "CURRENT_TIMESTAMP(6)"},
			{Name: "user_id", Type: "bigint unsigned", Nullable: false, References: "`users`(`id`)"},
		},
	})
	for _, want := range []string{"bigint unsigned", "varchar(255)", "json", "datetime(6)", "DEFAULT 'anonymous'", "DEFAULT CURRENT_TIMESTAMP(6)", "UNIQUE", "REFERENCES `users`(`id`)"} {
		if !strings.Contains(mysql, want) {
			t.Fatalf("expected mysql ddl to contain %q, got %s", want, mysql)
		}
	}
}
