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
	Columns   []string        `json:"columns" yaml:"columns"`
	Rows      [][]interface{} `json:"rows" yaml:"rows"`
	RowCount  int             `json:"row_count" yaml:"row_count"`
	ElapsedMS int64           `json:"elapsed_ms" yaml:"elapsed_ms"`
}

// StreamWriter renders row results incrementally so callers do not need to keep
// the full result set in memory before writing output.
type StreamWriter struct {
	w         io.Writer
	format    string
	columns   []string
	rows      [][]interface{}
	rowCount  int
	elapsedMS func() int64
	csv       *csv.Writer
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

// NewStreamWriter starts an incremental result writer for row-returning queries.
func NewStreamWriter(w io.Writer, format string, columns []string, elapsedMS func() int64) (*StreamWriter, error) {
	sw := &StreamWriter{w: w, format: strings.ToLower(format), columns: columns, elapsedMS: elapsedMS}
	switch sw.format {
	case "", "table":
		if _, err := fmt.Fprintln(w, strings.Join(columns, "\t")); err != nil {
			return nil, err
		}
	case "json":
		header, err := json.MarshalIndent(columns, "  ", "  ")
		if err != nil {
			return nil, err
		}
		if _, err := fmt.Fprintf(w, "{\n  \"columns\": %s,\n  \"rows\": [", header); err != nil {
			return nil, err
		}
	case "jsonl":
	case "csv", "tsv":
		sw.csv = csv.NewWriter(w)
		if sw.format == "tsv" {
			sw.csv.Comma = '\t'
		}
		if err := sw.csv.Write(columns); err != nil {
			return nil, err
		}
	case "markdown":
		if _, err := fmt.Fprintf(w, "| %s |\n", strings.Join(columns, " | ")); err != nil {
			return nil, err
		}
		separators := make([]string, len(columns))
		for i := range separators {
			separators[i] = "---"
		}
		if _, err := fmt.Fprintf(w, "| %s |\n", strings.Join(separators, " | ")); err != nil {
			return nil, err
		}
	case "yaml":
		sw.rows = [][]interface{}{}
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
	return sw, nil
}

// WriteRow writes one result row.
func (sw *StreamWriter) WriteRow(row []interface{}) error {
	switch sw.format {
	case "", "table":
		values := make([]string, len(row))
		for i, v := range row {
			values[i] = cellString(v)
		}
		if _, err := fmt.Fprintln(sw.w, strings.Join(values, "\t")); err != nil {
			return err
		}
	case "json":
		if sw.rowCount > 0 {
			if _, err := io.WriteString(sw.w, ","); err != nil {
				return err
			}
		}
		b, err := json.MarshalIndent(row, "    ", "  ")
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(sw.w, "\n    %s", b); err != nil {
			return err
		}
	case "jsonl":
		obj := map[string]interface{}{}
		for i, col := range sw.columns {
			if i < len(row) {
				obj[col] = row[i]
			}
		}
		if err := json.NewEncoder(sw.w).Encode(obj); err != nil {
			return err
		}
	case "csv", "tsv":
		record := make([]string, len(row))
		for i, v := range row {
			record[i] = cellString(v)
		}
		if err := sw.csv.Write(record); err != nil {
			return err
		}
	case "markdown":
		values := make([]string, len(row))
		for i, v := range row {
			values[i] = markdownCell(v)
		}
		if _, err := fmt.Fprintf(sw.w, "| %s |\n", strings.Join(values, " | ")); err != nil {
			return err
		}
	case "yaml":
		copied := append([]interface{}(nil), row...)
		sw.rows = append(sw.rows, copied)
	}
	sw.rowCount++
	return nil
}

// RowCount returns how many rows have been streamed so far.
func (sw *StreamWriter) RowCount() int {
	return sw.rowCount
}

// Close finalizes output and returns the result summary.
func (sw *StreamWriter) Close() (Result, error) {
	result := Result{Columns: sw.columns, RowCount: sw.rowCount, ElapsedMS: sw.elapsed()}
	switch sw.format {
	case "json":
		if _, err := fmt.Fprintf(sw.w, "\n  ],\n  \"row_count\": %d,\n  \"elapsed_ms\": %d\n}\n", result.RowCount, result.ElapsedMS); err != nil {
			return Result{}, err
		}
	case "csv", "tsv":
		sw.csv.Flush()
		if err := sw.csv.Error(); err != nil {
			return Result{}, err
		}
	case "yaml":
		result.Rows = sw.rows
		if err := yaml.NewEncoder(sw.w).Encode(result); err != nil {
			return Result{}, err
		}
	}
	return result, nil
}

func (sw *StreamWriter) elapsed() int64 {
	if sw.elapsedMS == nil {
		return 0
	}
	return sw.elapsedMS()
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
				record[i] = cellString(v)
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
			values[i] = markdownCell(v)
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
			values[i] = cellString(v)
		}
		if _, err := fmt.Fprintln(w, strings.Join(values, "\t")); err != nil {
			return err
		}
	}
	return nil
}

func cellString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func markdownCell(v interface{}) string {
	s := cellString(v)
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\r\n", "<br>")
	s = strings.ReplaceAll(s, "\n", "<br>")
	s = strings.ReplaceAll(s, "\r", "<br>")
	return s
}
