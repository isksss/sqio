package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

type Result struct {
	Columns   []string        `json:"columns" yaml:"columns"`
	Rows      [][]interface{} `json:"rows" yaml:"rows"`
	RowCount  int             `json:"row_count" yaml:"row_count"`
	ElapsedMS int64           `json:"elapsed_ms" yaml:"elapsed_ms"`
}

type LimitWriter struct {
	Writer io.Writer
	Limit  int
	wrote  int
}

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
