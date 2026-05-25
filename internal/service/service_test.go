package service

import (
	"context"
	"strings"
	"testing"
)

func TestExecSelectOne(t *testing.T) {
	result, err := Executor{}.Exec(context.Background(), "select 1", ExecOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 1 {
		t.Fatalf("expected 1 row, got %d", result.RowCount)
	}
}

func TestMetadataTables(t *testing.T) {
	tables, err := NewMetadataService().Tables(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) == 0 {
		t.Fatal("expected tables")
	}
}

func TestMetadataMermaidER(t *testing.T) {
	diagram, err := NewMetadataService().MermaidER(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diagram, "erDiagram") {
		t.Fatalf("unexpected diagram: %s", diagram)
	}
}
