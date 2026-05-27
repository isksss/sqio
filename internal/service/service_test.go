package service

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isksss/sqio/internal/db"
)

// TestExecSelectOne verifies the behavior covered by this test helper or case.
func TestExecSelectOne(t *testing.T) {
	result, err := Executor{}.Exec(context.Background(), "select 1", ExecOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 1 {
		t.Fatalf("expected 1 row, got %d", result.RowCount)
	}
}

// TestExecutorWriteSelectOne verifies the writer path used by CLI execution.
func TestExecutorWriteSelectOne(t *testing.T) {
	var buf bytes.Buffer
	result, err := Executor{}.Write(context.Background(), &buf, "select 1", ExecOptions{Format: "json"})
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 1 || !strings.Contains(buf.String(), `"row_count": 1`) {
		t.Fatalf("unexpected write result: %+v %s", result, buf.String())
	}
}

func TestExecutorConnectedWriteAndTransaction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	var buf bytes.Buffer
	result, err := Executor{}.Write(context.Background(), &buf, `
create table users (id integer primary key);
insert into users (id) values (1);
select id from users;
`, ExecOptions{Driver: "sqlite", DSN: path, Format: "json"})
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 1 || !strings.Contains(buf.String(), `"row_count": 1`) {
		t.Fatalf("unexpected connected write: %+v %s", result, buf.String())
	}
	buf.Reset()
	result, err = Executor{}.Write(context.Background(), &buf, `insert into users (id) values (2);`, ExecOptions{Driver: "sqlite", DSN: path, Format: "table", Transaction: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 1 || !strings.Contains(buf.String(), "OK (1 rows") {
		t.Fatalf("unexpected transaction write: %+v %s", result, buf.String())
	}
	buf.Reset()
	result, err = Executor{}.Write(context.Background(), &buf, `select 1`, ExecOptions{Format: "json", Transaction: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 1 || !strings.Contains(buf.String(), `"row_count": 1`) {
		t.Fatalf("unexpected demo transaction write: %+v %s", result, buf.String())
	}
	if _, err := (Executor{}).Write(context.Background(), serviceErrorWriter{}, `select 1`, ExecOptions{Format: "json"}); err == nil {
		t.Fatal("expected writer error")
	}
}

func TestExecutorErrors(t *testing.T) {
	if _, err := (Executor{}).Exec(context.Background(), "", ExecOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := (Executor{}).Exec(context.Background(), "select 2", ExecOptions{}); err == nil {
		t.Fatal("expected missing connection error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (Executor{}).Exec(ctx, "select 1", ExecOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled error, got %v", err)
	}
}

func TestTableDumpAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	if _, err := db.Execute(context.Background(), db.Config{Driver: "sqlite", DSN: path}, `create table users (id integer primary key, name text);`, db.ExecuteOptions{}); err != nil {
		t.Fatal(err)
	}
	result, err := LoadTable(context.Background(), strings.NewReader("id,name\n1,alice\n"), LoadOptions{Driver: "sqlite", DSN: path, Table: "users", Format: "csv"})
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsAffected != 1 {
		t.Fatalf("unexpected load result: %+v", result)
	}
	var buf bytes.Buffer
	dumped, err := DumpTable(context.Background(), &buf, DumpOptions{Driver: "sqlite", DSN: path, Table: "users", Format: "json"})
	if err != nil {
		t.Fatal(err)
	}
	if dumped.RowCount != 1 || !strings.Contains(buf.String(), `"row_count": 1`) {
		t.Fatalf("unexpected dump result=%+v output=%s", dumped, buf.String())
	}
	if _, err := LoadTable(context.Background(), strings.NewReader("id\n1\n"), LoadOptions{Driver: "sqlite", DSN: path, Table: "users", Format: "yaml"}); err == nil {
		t.Fatal("expected unsupported import format")
	}
	if _, err := LoadTable(context.Background(), strings.NewReader("id\n1\n"), LoadOptions{Driver: "sqlite", DSN: path, Table: "users", Format: "xml"}); err == nil || !strings.Contains(err.Error(), "unsupported import format") {
		t.Fatalf("expected unsupported import format error, got %v", err)
	}
	if err := ErrUnsupportedImportFormat("xml"); err == nil || !strings.Contains(err.Error(), "xml") {
		t.Fatalf("unexpected unsupported format error: %v", err)
	}
}

// TestMetadataTables verifies the behavior covered by this test helper or case.
func TestMetadataTables(t *testing.T) {
	tables, err := NewMetadataService().Tables(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) == 0 {
		t.Fatal("expected tables")
	}
}

// TestMetadataColumnsDDLAndSchema verifies fallback metadata lookup paths.
func TestMetadataColumnsDDLAndSchema(t *testing.T) {
	service := NewMetadataService()
	columns, err := service.Columns(context.Background(), "users")
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) == 0 || columns[0].Name != "id" {
		t.Fatalf("unexpected columns: %+v", columns)
	}
	ddl, err := service.DDL(context.Background(), "users")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ddl, "CREATE TABLE users") {
		t.Fatalf("unexpected ddl: %s", ddl)
	}
	indexes, err := service.Indexes(context.Background(), "users")
	if err != nil {
		t.Fatal(err)
	}
	if len(indexes) == 0 || !indexes[0].Primary {
		t.Fatalf("unexpected indexes: %+v", indexes)
	}
	schema, err := service.Schema(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Tables) != 2 {
		t.Fatalf("unexpected schema: %+v", schema)
	}
	schemas, err := service.Schemas(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(schemas) != 1 || schemas[0] != "default" {
		t.Fatalf("unexpected fallback schemas: %+v", schemas)
	}
}

func TestConnectedMetadataServiceSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	if _, err := db.Execute(context.Background(), db.Config{Driver: "sqlite", DSN: path}, `create table users (id integer primary key, name text); create unique index users_name_idx on users(name);`, db.ExecuteOptions{}); err != nil {
		t.Fatal(err)
	}
	service := NewConnectedMetadataService("sqlite", path)
	tables, err := service.Tables(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 || tables[0].Name != "users" {
		t.Fatalf("unexpected tables: %+v", tables)
	}
	columns, err := service.Columns(context.Background(), "users")
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 2 {
		t.Fatalf("unexpected columns: %+v", columns)
	}
	ddl, err := service.DDL(context.Background(), "users")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ddl, "CREATE TABLE users") {
		t.Fatalf("unexpected ddl: %s", ddl)
	}
	indexes, err := service.Indexes(context.Background(), "users")
	if err != nil {
		t.Fatal(err)
	}
	if len(indexes) == 0 {
		t.Fatalf("expected indexes: %+v", indexes)
	}
	schema, err := service.Schema(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Tables) != 1 {
		t.Fatalf("unexpected schema: %+v", schema)
	}
	schemas, err := service.Schemas(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(schemas) == 0 || schemas[0] != "main" {
		t.Fatalf("unexpected sqlite schemas: %+v", schemas)
	}
}

func TestConnectedMetadataServiceWithSchemaCarriesSchema(t *testing.T) {
	service := NewConnectedMetadataServiceWithSchema("sqlite", filepath.Join(t.TempDir(), "test.db"), "main")
	if service.db.Schema != "main" {
		t.Fatalf("expected schema on service db config, got %+v", service.db)
	}
}

func TestCompleteCandidates(t *testing.T) {
	service := NewMetadataService()
	completions, err := service.Complete(context.Background(), "select na", "users")
	if err != nil {
		t.Fatal(err)
	}
	if len(completions) != 1 || completions[0].Value != "name" || completions[0].Kind != "column" || completions[0].Table != "users" {
		t.Fatalf("unexpected column completions: %+v", completions)
	}
	completions, err = service.Complete(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, completion := range completions {
		kinds[completion.Kind] = true
	}
	if !kinds["keyword"] || !kinds["table"] || !kinds["column"] {
		t.Fatalf("expected keyword/table/column completions: %+v", completions)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Complete(ctx, "sel", ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled completion error, got %v", err)
	}
	if completionToken("select id, na") != "na" || completionToken(" \t\n") != "" {
		t.Fatal("unexpected completion token")
	}
	if !completionMatches("SELECT", "sel") || !completionMatches("users", "") || completionMatches("users", "rol") {
		t.Fatal("unexpected completion match result")
	}
}

func TestConnectedMetadataServiceMissingTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	if _, err := db.Execute(context.Background(), db.Config{Driver: "sqlite", DSN: path}, `create table users (id integer primary key);`, db.ExecuteOptions{}); err != nil {
		t.Fatal(err)
	}
	service := NewConnectedMetadataService("sqlite", path)
	if _, err := service.Columns(context.Background(), "missing"); err == nil {
		t.Fatal("expected missing connected columns error")
	}
	if _, err := service.Indexes(context.Background(), "missing"); err == nil {
		t.Fatal("expected missing connected indexes error")
	}
	if _, err := service.DDL(context.Background(), "missing"); err == nil {
		t.Fatal("expected missing connected ddl error")
	}
}

// TestMetadataErrors verifies missing tables and canceled contexts return errors.
func TestMetadataErrors(t *testing.T) {
	service := NewMetadataService()
	if _, err := service.Columns(context.Background(), "missing"); err == nil {
		t.Fatal("expected missing columns error")
	}
	if _, err := service.DDL(context.Background(), "missing"); err == nil {
		t.Fatal("expected missing ddl error")
	}
	if _, err := service.Indexes(context.Background(), "missing"); err == nil {
		t.Fatal("expected missing indexes error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Tables(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled tables error, got %v", err)
	}
	if _, err := service.Schema(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled schema error, got %v", err)
	}
	if _, err := service.Indexes(ctx, "users"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled indexes error, got %v", err)
	}
}

// TestMetadataMermaidER verifies the behavior covered by this test helper or case.
func TestMetadataMermaidER(t *testing.T) {
	diagram, err := NewMetadataService().MermaidER(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diagram, "erDiagram") {
		t.Fatalf("unexpected diagram: %s", diagram)
	}
}

type serviceErrorWriter struct{}

func (serviceErrorWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestMetadataComplete(t *testing.T) {
	service := NewMetadataService()
	candidates, err := service.Complete(context.Background(), "sel", "")
	if err != nil {
		t.Fatal(err)
	}
	foundSelect := false
	for _, candidate := range candidates {
		if candidate.Value == "SELECT" && candidate.Kind == "keyword" {
			foundSelect = true
		}
	}
	if !foundSelect {
		t.Fatalf("expected SELECT completion: %+v", candidates)
	}
	candidates, err = service.Complete(context.Background(), "na", "users")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].Value != "name" || candidates[0].Table != "users" {
		t.Fatalf("unexpected column completions: %+v", candidates)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Complete(ctx, "", ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled completion error, got %v", err)
	}
}

func TestDiffSchemas(t *testing.T) {
	from := Schema{Tables: []Table{{
		Name:    "users",
		Columns: []Column{{Name: "id", Type: "integer"}, {Name: "name", Type: "text"}},
		Indexes: []Index{{Name: "users_name_idx", Columns: []string{"name"}}},
	}, {
		Name: "old",
	}}}
	to := Schema{Tables: []Table{{
		Name:    "users",
		Columns: []Column{{Name: "id", Type: "bigint"}, {Name: "email", Type: "text"}},
		Indexes: []Index{{Name: "users_email_idx", Columns: []string{"email"}}},
	}, {
		Name: "new",
	}}}
	diff := DiffSchemas(from, to)
	types := map[string]bool{}
	for _, change := range diff.Changes {
		types[change.Type] = true
	}
	for _, typ := range []string{"drop_table", "add_table", "drop_column", "add_column", "change_column", "drop_index", "add_index"} {
		if !types[typ] {
			t.Fatalf("expected %s in diff: %+v", typ, diff.Changes)
		}
	}
}

// TestFromDBSchemaPreservesColumnMetadata verifies the behavior covered by this test helper or case.
func TestFromDBSchemaPreservesColumnMetadata(t *testing.T) {
	schema := fromDBSchema(db.SchemaInfo{Tables: []db.TableInfo{{
		Name: "users",
		Columns: []db.ColumnInfo{{
			Name: "email", Type: "text", Nullable: false, Unique: true, Default: "''", References: "profiles(email)",
		}},
		Indexes: []db.IndexInfo{{Name: "users_email_idx", Columns: []string{"email"}, Unique: true}},
		DDL:     "CREATE TABLE users (email text not null unique default '' references profiles(email));",
	}}})
	column := schema.Tables[0].Columns[0]
	if !column.Unique || column.Default != "''" || column.References != "profiles(email)" {
		t.Fatalf("column metadata was not preserved: %+v", column)
	}
	if len(schema.Tables[0].Indexes) != 1 || schema.Tables[0].Indexes[0].Name != "users_email_idx" {
		t.Fatalf("index metadata was not preserved: %+v", schema.Tables[0].Indexes)
	}
}
