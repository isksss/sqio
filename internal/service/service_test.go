package service

import (
	"bytes"
	"context"
	"errors"
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
	schema, err := service.Schema(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Tables) != 2 {
		t.Fatalf("unexpected schema: %+v", schema)
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
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Tables(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled tables error, got %v", err)
	}
	if _, err := service.Schema(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled schema error, got %v", err)
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

// TestFromDBSchemaPreservesColumnMetadata verifies the behavior covered by this test helper or case.
func TestFromDBSchemaPreservesColumnMetadata(t *testing.T) {
	schema := fromDBSchema(db.SchemaInfo{Tables: []db.TableInfo{{
		Name: "users",
		Columns: []db.ColumnInfo{{
			Name: "email", Type: "text", Nullable: false, Unique: true, Default: "''", References: "profiles(email)",
		}},
		DDL: "CREATE TABLE users (email text not null unique default '' references profiles(email));",
	}}})
	column := schema.Tables[0].Columns[0]
	if !column.Unique || column.Default != "''" || column.References != "profiles(email)" {
		t.Fatalf("column metadata was not preserved: %+v", column)
	}
}
