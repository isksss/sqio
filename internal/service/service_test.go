package service

import (
	"context"
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
