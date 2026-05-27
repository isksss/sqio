package output

import (
	"bytes"
	"encoding/json"
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

// TestWriteMarkdownEscapesCells verifies the behavior covered by this test helper or case.
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

// TestWriteYAML verifies the behavior covered by this test helper or case.
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

// TestWriteCSVUsesEmptyStringForNull verifies the behavior covered by this test helper or case.
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

// TestStreamWriterJSON verifies incremental JSON output keeps the public result
// shape while tracking row count.
func TestStreamWriterJSON(t *testing.T) {
	var buf bytes.Buffer
	writer, err := NewStreamWriter(&buf, "json", []string{"id"}, func() int64 { return 12 })
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteRow([]interface{}{1}); err != nil {
		t.Fatal(err)
	}
	result, err := writer.Close()
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 1 || result.ElapsedMS != 12 {
		t.Fatalf("unexpected result: %+v", result)
	}
	var decoded Result
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, buf.String())
	}
	if decoded.RowCount != 1 {
		t.Fatalf("unexpected json result: %+v", decoded)
	}
}

// TestStreamWriterFormats verifies row streaming for the text-oriented formats.
func TestStreamWriterFormats(t *testing.T) {
	cases := map[string]string{
		"table":    "id\n1\n",
		"csv":      "id\n1\n",
		"tsv":      "id\n1\n",
		"jsonl":    "{\"id\":1}\n",
		"markdown": "| id |\n| --- |\n| 1 |\n",
	}
	for format, want := range cases {
		t.Run(format, func(t *testing.T) {
			var buf bytes.Buffer
			writer, err := NewStreamWriter(&buf, format, []string{"id"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := writer.WriteRow([]interface{}{1}); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			if buf.String() != want {
				t.Fatalf("unexpected %s output: %q", format, buf.String())
			}
		})
	}
}

// TestStreamWriterYAML verifies the YAML fallback preserves the Result shape.
func TestStreamWriterYAML(t *testing.T) {
	var buf bytes.Buffer
	writer, err := NewStreamWriter(&buf, "yaml", []string{"id"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteRow([]interface{}{1}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "row_count: 1") {
		t.Fatalf("unexpected yaml: %s", buf.String())
	}
}

// TestWriteTableAndUnsupportedFormat covers table summaries and invalid formats.
func TestWriteTableAndUnsupportedFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "table", Result{RowCount: 3, ElapsedMS: 5}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "OK (3 rows, 5 ms)") {
		t.Fatalf("unexpected table summary: %s", buf.String())
	}
	if err := Write(&buf, "unknown", Result{}); err == nil {
		t.Fatal("expected unsupported format error")
	}
}
