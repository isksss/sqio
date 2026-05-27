// Package output serializes query results into CLI-friendly formats.
package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// Result is the database execution payload shared by output encoders.
// Columns is empty for statements that do not return rows, while RowCount and
// ElapsedMS still describe the completed operation.
type Result struct {
	Columns   []string        `json:"columns"`
	Rows      [][]interface{} `json:"rows"`
	RowCount  int             `json:"row_count"`
	ElapsedMS int64           `json:"elapsed_ms"`
}

// LimitWriter wraps another writer and fails once the configured byte limit is
// exceeded. A non-positive Limit disables the guard and passes writes through.
type LimitWriter struct {
	Writer io.Writer
	Limit  int
	wrote  int
}

// Write writes p to the underlying writer while accounting for the configured
// byte budget. If p would exceed the limit, it writes the remaining bytes and
// returns an error so callers can stop producing output.
func (w *LimitWriter) Write(p []byte) (int, error) {
	if w.Limit <= 0 {
		return w.Writer.Write(p)
	}
	remaining := w.Limit - w.wrote
	if remaining <= 0 {
		return 0, fmt.Errorf("output exceeded max bytes")
	}
	if len(p) > remaining {
		n, err := w.Writer.Write(p[:remaining])
		w.wrote += n
		if err != nil {
			return n, err
		}
		return n, fmt.Errorf("output exceeded max bytes")
	}
	n, err := w.Writer.Write(p)
	w.wrote += n
	return n, err
}

// Write renders result to w using the requested format. The empty format is
// treated as "table" so CLI callers get a readable default without extra flags.
func Write(w io.Writer, format string, result Result) error {
	switch strings.ToLower(format) {
	case "", "table":
		return writeTable(w, result)
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	case "yaml":
		return yaml.NewEncoder(w).Encode(result)
	case "markdown":
		return writeMarkdown(w, result)
	case "jsonl":
		for _, row := range result.Rows {
			obj := map[string]interface{}{}
			for i, col := range result.Columns {
				if i < len(row) {
					obj[col] = row[i]
				}
			}
			if err := json.NewEncoder(w).Encode(obj); err != nil {
				return err
			}
		}
		return nil
	case "csv", "tsv":
		writer := csv.NewWriter(w)
		if strings.ToLower(format) == "tsv" {
			writer.Comma = '\t'
		}
		if err := writer.Write(result.Columns); err != nil {
			return err
		}
		for _, row := range result.Rows {
			record := make([]string, len(row))
			for i, v := range row {
				record[i] = fmt.Sprint(v)
			}
			if err := writer.Write(record); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// writeMarkdown renders rows as a GitHub-flavored Markdown table and renders
// non-row results as a compact execution summary.
func writeMarkdown(w io.Writer, result Result) error {
	if len(result.Columns) == 0 {
		_, err := fmt.Fprintf(w, "OK (%d rows, %d ms)\n", result.RowCount, result.ElapsedMS)
		return err
	}
	if _, err := fmt.Fprintf(w, "| %s |\n", strings.Join(result.Columns, " | ")); err != nil {
		return err
	}
	separators := make([]string, len(result.Columns))
	for i := range separators {
		separators[i] = "---"
	}
	if _, err := fmt.Fprintf(w, "| %s |\n", strings.Join(separators, " | ")); err != nil {
		return err
	}
	for _, row := range result.Rows {
		values := make([]string, len(row))
		for i, v := range row {
			values[i] = fmt.Sprint(v)
		}
		if _, err := fmt.Fprintf(w, "| %s |\n", strings.Join(values, " | ")); err != nil {
			return err
		}
	}
	return nil
}

// writeTable renders rows as tab-separated text for terminals and simple pipes.
// Non-row statements use the same execution summary as the Markdown encoder.
func writeTable(w io.Writer, result Result) error {
	if len(result.Columns) == 0 {
		_, err := fmt.Fprintf(w, "OK (%d rows, %d ms)\n", result.RowCount, result.ElapsedMS)
		return err
	}
	if _, err := fmt.Fprintln(w, strings.Join(result.Columns, "\t")); err != nil {
		return err
	}
	for _, row := range result.Rows {
		values := make([]string, len(row))
		for i, v := range row {
			values[i] = fmt.Sprint(v)
		}
		if _, err := fmt.Fprintln(w, strings.Join(values, "\t")); err != nil {
			return err
		}
	}
	return nil
}
