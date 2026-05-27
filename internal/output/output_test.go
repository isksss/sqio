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

func TestWriteAdditionalFormats(t *testing.T) {
	cases := map[string]string{
		"table": "id\tname\n1\talice\n",
		"tsv":   "id\tname\n1\talice\n",
		"jsonl": "{\"id\":1,\"name\":\"alice\"}\n",
	}
	for format, want := range cases {
		t.Run(format, func(t *testing.T) {
			var buf bytes.Buffer
			err := Write(&buf, format, Result{Columns: []string{"id", "name"}, Rows: [][]interface{}{{1, "alice"}}, RowCount: 1})
			if err != nil {
				t.Fatal(err)
			}
			if buf.String() != want {
				t.Fatalf("unexpected %s: %q", format, buf.String())
			}
		})
	}
}

func TestWriteSummaries(t *testing.T) {
	for _, format := range []string{"markdown", ""} {
		t.Run(format, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Write(&buf, format, Result{RowCount: 2, ElapsedMS: 3}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(buf.String(), "OK (2 rows, 3 ms)") {
				t.Fatalf("unexpected summary: %q", buf.String())
			}
		})
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

func TestLimitWriterPassThroughAndExactLimit(t *testing.T) {
	var buf bytes.Buffer
	writer := &LimitWriter{Writer: &buf}
	n, err := writer.Write([]byte("abc"))
	if err != nil || n != 3 || buf.String() != "abc" {
		t.Fatalf("unexpected pass-through write n=%d err=%v buf=%q", n, err, buf.String())
	}
	buf.Reset()
	writer = &LimitWriter{Writer: &buf, Limit: 3}
	if _, err = writer.Write([]byte("def")); err != nil {
		t.Fatal(err)
	}
	n, err = writer.Write([]byte("g"))
	if err == nil || n != 0 {
		t.Fatalf("expected exhausted limit without write, got n=%d err=%v", n, err)
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

func TestStreamWriterUnsupportedFormat(t *testing.T) {
	if _, err := NewStreamWriter(&bytes.Buffer{}, "bad", []string{"id"}, nil); err == nil {
		t.Fatal("expected unsupported format error")
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
			if writer.RowCount() != 1 {
				t.Fatalf("unexpected row count before close: %d", writer.RowCount())
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

func TestStreamWriterDefaultFormatAndElapsedNil(t *testing.T) {
	var buf bytes.Buffer
	writer, err := NewStreamWriter(&buf, "", []string{"id"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteRow([]interface{}{[]byte("abc")}); err != nil {
		t.Fatal(err)
	}
	result, err := writer.Close()
	if err != nil {
		t.Fatal(err)
	}
	if result.ElapsedMS != 0 || result.RowCount != 1 || !strings.Contains(buf.String(), "[97 98 99]") {
		t.Fatalf("unexpected stream result=%+v output=%q", result, buf.String())
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

func TestWritePropagatesWriterErrors(t *testing.T) {
	result := Result{Columns: []string{"id"}, Rows: [][]interface{}{{1}}, RowCount: 1}
	for _, format := range []string{"table", "markdown", "jsonl", "csv"} {
		t.Run(format, func(t *testing.T) {
			if err := Write(errorWriter{}, format, result); err == nil {
				t.Fatal("expected writer error")
			}
		})
	}
	if err := Write(errorWriter{}, "table", Result{RowCount: 1}); err == nil {
		t.Fatal("expected table summary writer error")
	}
	if err := Write(errorWriter{}, "markdown", Result{RowCount: 1}); err == nil {
		t.Fatal("expected markdown summary writer error")
	}
}

func TestStreamWriterPropagatesWriterErrors(t *testing.T) {
	if _, err := NewStreamWriter(errorWriter{}, "table", []string{"id"}, nil); err == nil {
		t.Fatal("expected table header writer error")
	}
	if _, err := NewStreamWriter(errorWriter{}, "json", []string{"id"}, nil); err == nil {
		t.Fatal("expected json header writer error")
	}
	if _, err := NewStreamWriter(errorWriter{}, "markdown", []string{"id"}, nil); err == nil {
		t.Fatal("expected markdown header writer error")
	}
	var buf bytes.Buffer
	writer, err := NewStreamWriter(&buf, "json", []string{"bad"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	writer.w = errorWriter{}
	if err := writer.WriteRow([]interface{}{1}); err == nil {
		t.Fatal("expected json row writer error")
	}
	writer, err = NewStreamWriter(&buf, "yaml", []string{"id"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	writer.w = errorWriter{}
	if _, err := writer.Close(); err == nil {
		t.Fatal("expected yaml close writer error")
	}
	writer, err = NewStreamWriter(&buf, "table", []string{"id"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	writer.w = errorWriter{}
	if err := writer.WriteRow([]interface{}{1}); err == nil {
		t.Fatal("expected table row writer error")
	}
	writer, err = NewStreamWriter(&buf, "markdown", []string{"id"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	writer.w = errorWriter{}
	if err := writer.WriteRow([]interface{}{1}); err == nil {
		t.Fatal("expected markdown row writer error")
	}
	writer, err = NewStreamWriter(&buf, "json", []string{"id"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteRow([]interface{}{1}); err != nil {
		t.Fatal(err)
	}
	writer.w = errorWriter{}
	if err := writer.WriteRow([]interface{}{2}); err == nil {
		t.Fatal("expected json comma writer error")
	}
	writer, err = NewStreamWriter(errorWriter{}, "csv", []string{"id"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteRow([]interface{}{1}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Close(); err == nil {
		t.Fatal("expected csv close writer error")
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) {
	return 0, outputAssertErr("write failed")
}

type outputAssertErr string

func (e outputAssertErr) Error() string { return string(e) }
