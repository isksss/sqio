package db

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isksss/sqio/internal/output"
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

func TestOpenNormalizeErrorsAndAliases(t *testing.T) {
	if _, _, err := Open(context.Background(), Config{Driver: "sqlite"}); err == nil {
		t.Fatal("expected sqlite dsn error")
	}
	if _, _, err := Open(context.Background(), Config{Driver: "duckdb"}); err == nil {
		t.Fatal("expected duckdb dsn error")
	}
	if _, _, err := Open(context.Background(), Config{Driver: "postgres"}); err == nil {
		t.Fatal("expected postgres dsn error")
	}
	if _, _, err := Open(context.Background(), Config{Driver: "mysql"}); err == nil {
		t.Fatal("expected mysql dsn error")
	}
	if driver, _, err := normalize(Config{Driver: "cockroachdb", DSN: "postgres://example"}); err != nil || driver != "pgx" {
		t.Fatalf("unexpected cockroach normalize: %s %v", driver, err)
	}
	if driver, _, err := normalize(Config{Driver: "tidb", DSN: "user@tcp(localhost:4000)/test"}); err != nil || driver != "mysql" {
		t.Fatalf("unexpected tidb normalize: %s %v", driver, err)
	}
	for input, want := range map[string]string{
		"mssql":      "sqlserver",
		"sqlserver":  "sqlserver",
		"oracle":     "oracle",
		"clickhouse": "clickhouse",
		"ch":         "clickhouse",
		"duckdb":     "duckdb",
	} {
		if driver, _, err := normalize(Config{Driver: input, DSN: "dsn"}); err != nil || driver != want {
			t.Fatalf("unexpected normalize for %s: %s %v", input, driver, err)
		}
	}
	for _, input := range []string{"sqlserver", "oracle", "clickhouse"} {
		if _, _, err := normalize(Config{Driver: input}); err == nil {
			t.Fatalf("expected missing dsn error for %s", input)
		}
	}
	if _, _, err := Open(context.Background(), Config{Driver: "unknown", DSN: "x"}); err == nil {
		t.Fatal("expected unsupported driver error")
	}
}

func TestExecuteToWriterEmptyAndBadFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	var buf bytes.Buffer
	result, err := ExecuteToWriter(context.Background(), Config{Driver: "sqlite", DSN: path}, "", ExecuteOptions{}, &buf, "table")
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 0 || !strings.Contains(buf.String(), "OK (0 rows") {
		t.Fatalf("unexpected empty result: %+v %s", result, buf.String())
	}
	buf.Reset()
	if _, err := ExecuteToWriter(context.Background(), Config{Driver: "sqlite", DSN: path}, "select 1", ExecuteOptions{}, &buf, "bad"); err == nil {
		t.Fatal("expected bad format error")
	}
}

func TestReturnsRows(t *testing.T) {
	for _, sql := range []string{"select 1", "with x as (select 1) select * from x", "show tables", "describe users", "explain select 1", "pragma table_info(users)"} {
		if !returnsRows(sql) {
			t.Fatalf("expected row-returning SQL: %s", sql)
		}
	}
	if returnsRows("insert into users values (1)") {
		t.Fatal("expected insert to be non-row statement")
	}
	if placeholder("sqlserver", 2) != "@p2" || placeholder("oracle", 2) != ":2" || placeholder("clickhouse", 2) != "?" {
		t.Fatal("unexpected driver placeholder")
	}
}

// TestExecuteToWriterStreamsLastRows verifies streaming output writes the final
// row-returning statement directly to the supplied writer.
func TestExecuteToWriterStreamsLastRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	var buf bytes.Buffer
	result, err := ExecuteToWriter(context.Background(), Config{Driver: "sqlite", DSN: path}, `
create table users (id integer primary key, name text);
insert into users (name) values ('alice'), ('bob');
select id, name from users order by id;
`, ExecuteOptions{MaxRows: 1}, &buf, "json")
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 1 {
		t.Fatalf("expected streamed row count 1, got %d", result.RowCount)
	}
	var decoded output.Result
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, buf.String())
	}
	if decoded.RowCount != 1 || len(decoded.Rows) != 1 {
		t.Fatalf("unexpected streamed json summary: %+v", decoded)
	}
}

// TestExecuteToWriterWritesExecSummary verifies non-row final statements still
// use the regular result summary shape.
func TestExecuteToWriterWritesExecSummary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	var buf bytes.Buffer
	result, err := ExecuteToWriter(context.Background(), Config{Driver: "sqlite", DSN: path}, `
create table users (id integer primary key);
insert into users (id) values (1);
`, ExecuteOptions{}, &buf, "table")
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 1 {
		t.Fatalf("expected affected row count, got %d", result.RowCount)
	}
	if !strings.Contains(buf.String(), "OK (1 rows") {
		t.Fatalf("unexpected summary: %s", buf.String())
	}
}

func TestExecuteToWriterTransactionAndEarlierRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	var buf bytes.Buffer
	result, err := ExecuteToWriter(context.Background(), Config{Driver: "sqlite", DSN: path}, `
create table users (id integer primary key, name text);
select 1;
insert into users (id, name) values (1, 'alice');
`, ExecuteOptions{Transaction: true}, &buf, "json")
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 1 || !strings.Contains(buf.String(), `"row_count": 1`) {
		t.Fatalf("unexpected transaction writer result=%+v output=%s", result, buf.String())
	}
	queryResult, err := Execute(context.Background(), Config{Driver: "sqlite", DSN: path}, `select name from users`, ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if queryResult.RowCount != 1 || queryResult.Rows[0][0] != "alice" {
		t.Fatalf("expected committed row, got %+v", queryResult)
	}
}

func TestExecuteToWriterErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	var buf bytes.Buffer
	if _, err := ExecuteToWriter(context.Background(), Config{Driver: "sqlite", DSN: path}, "select * from missing", ExecuteOptions{}, &buf, "json"); err == nil {
		t.Fatal("expected query error")
	}
	if _, err := ExecuteToWriter(context.Background(), Config{Driver: "sqlite", DSN: path}, "create table users (id integer primary key); insert into missing values (1)", ExecuteOptions{}, &buf, "json"); err == nil {
		t.Fatal("expected exec error")
	}
	if _, err := ExecuteToWriter(context.Background(), Config{Driver: "sqlite", DSN: path}, "select 1", ExecuteOptions{}, errorWriter{}, "json"); err == nil {
		t.Fatal("expected writer error")
	}
}

func TestDumpTableAndLoadCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	if _, err := Execute(context.Background(), Config{Driver: "sqlite", DSN: path}, `create table users (id integer primary key, name text);`, ExecuteOptions{}); err != nil {
		t.Fatal(err)
	}
	result, err := LoadCSV(context.Background(), Config{Driver: "sqlite", DSN: path}, "users", strings.NewReader("id,name\n1,alice\n2,bob\n"))
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsAffected != 2 {
		t.Fatalf("expected 2 loaded rows, got %d", result.RowsAffected)
	}
	var buf bytes.Buffer
	dumped, err := DumpTable(context.Background(), Config{Driver: "sqlite", DSN: path}, "users", ExecuteOptions{MaxRows: 1}, &buf, "csv")
	if err != nil {
		t.Fatal(err)
	}
	if dumped.RowCount != 1 || !strings.Contains(buf.String(), "id,name\n1,alice\n") {
		t.Fatalf("unexpected dump result=%+v output=%q", dumped, buf.String())
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) {
	return 0, dbAssertErr("write failed")
}

type dbAssertErr string

func (e dbAssertErr) Error() string { return string(e) }

func TestLoadJSONAndJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	cfg := Config{Driver: "sqlite", DSN: path}
	if _, err := Execute(context.Background(), cfg, `create table users (id integer primary key, name text);`, ExecuteOptions{}); err != nil {
		t.Fatal(err)
	}
	result, err := LoadJSON(context.Background(), cfg, "users", strings.NewReader(`{"columns":["id","name"],"rows":[[1,"alice"]]}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsAffected != 1 {
		t.Fatalf("expected 1 json row, got %d", result.RowsAffected)
	}
	result, err = LoadJSON(context.Background(), cfg, "users", strings.NewReader(`[{"id":2,"name":"bob"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsAffected != 1 {
		t.Fatalf("expected 1 json object row, got %d", result.RowsAffected)
	}
	result, err = LoadJSONL(context.Background(), cfg, "users", strings.NewReader("{\"id\":3,\"name\":\"carol\"}\n\n"))
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsAffected != 1 {
		t.Fatalf("expected 1 jsonl row, got %d", result.RowsAffected)
	}
	queryResult, err := Execute(context.Background(), cfg, "select name from users order by id", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if queryResult.RowCount != 3 || queryResult.Rows[2][0] != "carol" {
		t.Fatalf("unexpected loaded rows: %+v", queryResult)
	}
}

func TestLoadYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	cfg := Config{Driver: "sqlite", DSN: path}
	if _, err := Execute(context.Background(), cfg, `create table users (id integer primary key, name text);`, ExecuteOptions{}); err != nil {
		t.Fatal(err)
	}
	result, err := LoadYAML(context.Background(), cfg, "users", strings.NewReader("columns: [id, name]\nrows:\n  - [1, alice]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsAffected != 1 {
		t.Fatalf("expected 1 yaml result row, got %d", result.RowsAffected)
	}
	result, err = LoadYAML(context.Background(), cfg, "users", strings.NewReader("- id: 2\n  name: bob\n"))
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsAffected != 1 {
		t.Fatalf("expected 1 yaml object row, got %d", result.RowsAffected)
	}
	queryResult, err := Execute(context.Background(), cfg, "select name from users order by id", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if queryResult.RowCount != 2 || queryResult.Rows[1][0] != "bob" {
		t.Fatalf("unexpected yaml loaded rows: %+v", queryResult)
	}
}

func TestLoadYAMLError(t *testing.T) {
	cfg := Config{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "test.db")}
	if _, err := LoadYAML(context.Background(), cfg, "users", strings.NewReader(": bad")); err == nil {
		t.Fatal("expected bad yaml error")
	}
}

func TestLoadJSONErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	cfg := Config{Driver: "sqlite", DSN: path}
	if _, err := LoadJSON(context.Background(), cfg, "users", strings.NewReader(`not-json`)); err == nil {
		t.Fatal("expected bad json error")
	}
	if _, err := LoadJSONL(context.Background(), cfg, "users", strings.NewReader(`not-json`)); err == nil {
		t.Fatal("expected bad jsonl error")
	}
	if _, err := loadRows(context.Background(), cfg, "users", nil, nil); err == nil {
		t.Fatal("expected missing columns error")
	}
	if _, err := loadRows(context.Background(), cfg, "users", []string{"id"}, [][]interface{}{{1, 2}}); err == nil {
		t.Fatal("expected wrong row width error")
	}
	if _, err := loadRows(context.Background(), cfg, "users", []string{""}, [][]interface{}{{1}}); err == nil {
		t.Fatal("expected empty column error")
	}
}

func TestLoadCSVErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	if _, err := Execute(context.Background(), Config{Driver: "sqlite", DSN: path}, `create table users (id integer primary key, name text);`, ExecuteOptions{}); err != nil {
		t.Fatal(err)
	}
	for name, input := range map[string]string{
		"empty header":       "",
		"empty column":       "id,\n1,a\n",
		"wrong record width": "id,name\n1\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadCSV(context.Background(), Config{Driver: "sqlite", DSN: path}, "users", strings.NewReader(input)); err == nil {
				t.Fatal("expected load error")
			}
		})
	}
	if _, err := LoadCSV(context.Background(), Config{Driver: "sqlite", DSN: path}, "", strings.NewReader("id\n1\n")); err == nil {
		t.Fatal("expected missing table error")
	}
	var buf bytes.Buffer
	if _, err := DumpTable(context.Background(), Config{Driver: "sqlite", DSN: path}, "", ExecuteOptions{}, &buf, "csv"); err == nil {
		t.Fatal("expected missing dump table error")
	}
	if _, err := DumpTable(context.Background(), Config{Driver: "sqlite", DSN: path}, "missing", ExecuteOptions{}, &buf, "csv"); err == nil {
		t.Fatal("expected missing dump table query error")
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
	schemas, err := Schemas(context.Background(), Config{Driver: "sqlite", DSN: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(schemas) == 0 || schemas[0] != "main" {
		t.Fatalf("unexpected sqlite schemas: %+v", schemas)
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
