package output

import (
	"bytes"
	"strings"
	"testing"
)

// TestWriteJSON verifies the behavior covered by this test helper or case.
func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, "json", Result{Columns: []string{"id"}, Rows: [][]interface{}{{1}}, RowCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"row_count": 1`) {
		t.Fatalf("unexpected json: %s", buf.String())
	}
}

// TestWriteMarkdown verifies the behavior covered by this test helper or case.
func TestWriteMarkdown(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, "markdown", Result{Columns: []string{"id"}, Rows: [][]interface{}{{1}}, RowCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "| id |") {
		t.Fatalf("unexpected markdown: %s", buf.String())
	}
}

// TestWriteYAML verifies the behavior covered by this test helper or case.
func TestWriteYAML(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, "yaml", Result{Columns: []string{"id"}, Rows: [][]interface{}{{1}}, RowCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "rowcount: 1") {
		t.Fatalf("unexpected yaml: %s", buf.String())
	}
}

// TestLimitWriter verifies the behavior covered by this test helper or case.
func TestLimitWriter(t *testing.T) {
	var buf bytes.Buffer
	writer := &LimitWriter{Writer: &buf, Limit: 3}
	_, err := writer.Write([]byte("abcd"))
	if err == nil {
		t.Fatal("expected max bytes error")
	}
	if buf.String() != "abc" {
		t.Fatalf("expected truncated output, got %q", buf.String())
	}
}
