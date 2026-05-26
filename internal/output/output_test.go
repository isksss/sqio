package output

import (
	"bytes"
	"strings"
	"testing"
)

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

func TestWriteMarkdownEscapesCells(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, "markdown", Result{Columns: []string{"note"}, Rows: [][]interface{}{{"a|b\nc"}}, RowCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `a\|b<br>c`) {
		t.Fatalf("unexpected markdown: %s", buf.String())
	}
}

func TestWriteYAML(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, "yaml", Result{Columns: []string{"id"}, Rows: [][]interface{}{{1}}, RowCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "row_count: 1") {
		t.Fatalf("unexpected yaml: %s", buf.String())
	}
}

func TestWriteCSVUsesEmptyStringForNull(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, "csv", Result{Columns: []string{"id", "name"}, Rows: [][]interface{}{{1, nil}}, RowCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "1,\n") {
		t.Fatalf("unexpected csv: %q", buf.String())
	}
}

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
