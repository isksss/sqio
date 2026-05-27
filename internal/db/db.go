// Package db opens database connections and executes SQL against supported
// drivers.
package db

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	_ "github.com/go-sql-driver/mysql"
	"github.com/isksss/sqio/internal/dbdriver"
	"github.com/isksss/sqio/internal/output"
	"github.com/isksss/sqio/internal/query"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/marcboeker/go-duckdb"
	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/sijms/go-ora/v2"
	"gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"
)

// Config contains the normalized driver name and DSN needed to open a database.
type Config struct {
	Driver string
	DSN    string
	Schema string
}

// ExecuteOptions controls optional execution behavior for a SQL request.
type ExecuteOptions struct {
	MaxRows     int
	Explain     bool
	Analyze     bool
	Transaction bool
}

// ImportResult summarizes a table import operation.
type ImportResult struct {
	RowsAffected int `json:"rows_affected"`
}

// Open validates cfg, opens the matching database driver, and verifies the
// connection with PingContext. The returned driver is the normalized driver name.
func Open(ctx context.Context, cfg Config) (*sql.DB, string, error) {
	driver, dsn, err := normalize(cfg)
	if err != nil {
		return nil, "", err
	}
	conn, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, "", err
	}
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, "", err
	}
	return conn, driver, nil
}

var openConnection = Open

// Execute runs each parsed statement in sqlText and returns the result from the
// last statement. Row-returning statements are scanned into output.Result, while
// write statements report affected row count.
func Execute(ctx context.Context, cfg Config, sqlText string, opts ExecuteOptions) (output.Result, error) {
	started := time.Now()
	conn, driver, err := Open(ctx, cfg)
	if err != nil {
		return output.Result{}, err
	}
	defer conn.Close()

	if opts.Explain {
		sqlText = explainSQL(driver, sqlText, opts.Analyze)
	}
	var execer interface {
		QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
		ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	} = conn
	var tx *sql.Tx
	if opts.Transaction {
		tx, err = conn.BeginTx(ctx, nil)
		if err != nil {
			return output.Result{}, err
		}
		defer tx.Rollback()
		execer = tx
	}
	result := output.Result{ElapsedMS: elapsed(started)}
	for _, statement := range query.Statements(sqlText) {
		if returnsRows(statement) {
			rows, err := execer.QueryContext(ctx, statement)
			if err != nil {
				return output.Result{}, err
			}
			scanned, scanErr := scanRows(rows, opts.MaxRows)
			_ = rows.Close()
			if scanErr != nil {
				return output.Result{}, scanErr
			}
			scanned.ElapsedMS = elapsed(started)
			result = scanned
			continue
		}
		execResult, execErr := execer.ExecContext(ctx, statement)
		if execErr != nil {
			return output.Result{}, execErr
		}
		affected, _ := execResult.RowsAffected()
		result = output.Result{RowCount: int(affected), ElapsedMS: elapsed(started)}
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return output.Result{}, err
		}
	}
	return result, nil
}

// ExecuteToWriter runs SQL and streams the last row-returning statement directly
// to w. Earlier statements are executed for side effects only, matching Execute's
// "last statement is the result" behavior.
func ExecuteToWriter(ctx context.Context, cfg Config, sqlText string, opts ExecuteOptions, w io.Writer, format string) (output.Result, error) {
	started := time.Now()
	conn, driver, err := Open(ctx, cfg)
	if err != nil {
		return output.Result{}, err
	}
	defer conn.Close()

	if opts.Explain {
		sqlText = explainSQL(driver, sqlText, opts.Analyze)
	}
	statements := query.Statements(sqlText)
	if len(statements) == 0 {
		result := output.Result{ElapsedMS: elapsed(started)}
		if err := output.Write(w, format, result); err != nil {
			return output.Result{}, err
		}
		return result, nil
	}
	var execer interface {
		QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
		ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	} = conn
	var tx *sql.Tx
	if opts.Transaction {
		tx, err = conn.BeginTx(ctx, nil)
		if err != nil {
			return output.Result{}, err
		}
		defer tx.Rollback()
		execer = tx
	}
	result := output.Result{ElapsedMS: elapsed(started)}
	for i, statement := range statements {
		last := i == len(statements)-1
		if returnsRows(statement) {
			rows, err := execer.QueryContext(ctx, statement)
			if err != nil {
				return output.Result{}, err
			}
			if last {
				result, err = streamRows(rows, opts.MaxRows, w, format, started)
			} else {
				result, err = scanRows(rows, opts.MaxRows)
			}
			_ = rows.Close()
			if err != nil {
				return output.Result{}, err
			}
			result.ElapsedMS = elapsed(started)
			continue
		}
		execResult, execErr := execer.ExecContext(ctx, statement)
		if execErr != nil {
			return output.Result{}, execErr
		}
		affected, _ := execResult.RowsAffected()
		result = output.Result{RowCount: int(affected), ElapsedMS: elapsed(started)}
		if last {
			if err := output.Write(w, format, result); err != nil {
				return output.Result{}, err
			}
		}
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return output.Result{}, err
		}
	}
	return result, nil
}

// DumpTable streams all rows from tableName to w using the requested output
// format.
func DumpTable(ctx context.Context, cfg Config, tableName string, opts ExecuteOptions, w io.Writer, format string) (output.Result, error) {
	conn, driver, err := Open(ctx, cfg)
	if err != nil {
		return output.Result{}, err
	}
	defer conn.Close()
	if strings.TrimSpace(tableName) == "" {
		return output.Result{}, fmt.Errorf("table is required")
	}
	started := time.Now()
	rows, err := conn.QueryContext(ctx, "select * from "+quoteIdent(driver, tableName))
	if err != nil {
		return output.Result{}, err
	}
	defer rows.Close()
	return streamRows(rows, opts.MaxRows, w, format, started)
}

// LoadCSV inserts CSV rows into tableName. The first CSV record is treated as a
// header containing destination column names.
func LoadCSV(ctx context.Context, cfg Config, tableName string, r io.Reader) (ImportResult, error) {
	conn, driver, err := Open(ctx, cfg)
	if err != nil {
		return ImportResult{}, err
	}
	defer conn.Close()
	if strings.TrimSpace(tableName) == "" {
		return ImportResult{}, fmt.Errorf("table is required")
	}
	reader := csv.NewReader(r)
	header, err := reader.Read()
	if err != nil {
		return ImportResult{}, err
	}
	if len(header) == 0 {
		return ImportResult{}, fmt.Errorf("csv header is required")
	}
	columns := make([]string, len(header))
	placeholders := make([]string, len(header))
	for i, column := range header {
		if strings.TrimSpace(column) == "" {
			return ImportResult{}, fmt.Errorf("csv header contains empty column")
		}
		columns[i] = quoteIdent(driver, column)
		placeholders[i] = placeholder(driver, i+1)
	}
	stmtSQL := fmt.Sprintf("insert into %s (%s) values (%s)", quoteIdent(driver, tableName), strings.Join(columns, ","), strings.Join(placeholders, ","))
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return ImportResult{}, err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, stmtSQL)
	if err != nil {
		return ImportResult{}, err
	}
	defer stmt.Close()
	result := ImportResult{}
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ImportResult{}, err
		}
		if len(record) != len(header) {
			return ImportResult{}, fmt.Errorf("csv record has %d fields, want %d", len(record), len(header))
		}
		values := make([]interface{}, len(record))
		for i, value := range record {
			values[i] = value
		}
		if _, err := stmt.ExecContext(ctx, values...); err != nil {
			return ImportResult{}, err
		}
		result.RowsAffected++
	}
	if err := tx.Commit(); err != nil {
		return ImportResult{}, err
	}
	return result, nil
}

// LoadJSON inserts rows from either output.Result JSON or an array of objects.
func LoadJSON(ctx context.Context, cfg Config, tableName string, r io.Reader) (ImportResult, error) {
	var raw json.RawMessage
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return ImportResult{}, err
	}
	var result output.Result
	if err := json.Unmarshal(raw, &result); err == nil && len(result.Columns) > 0 {
		return loadRows(ctx, cfg, tableName, result.Columns, result.Rows)
	}
	var objects []map[string]interface{}
	if err := json.Unmarshal(raw, &objects); err != nil {
		return ImportResult{}, err
	}
	rows, columns := rowsFromObjects(objects)
	return loadRows(ctx, cfg, tableName, columns, rows)
}

// LoadJSONL inserts newline-delimited JSON objects.
func LoadJSONL(ctx context.Context, cfg Config, tableName string, r io.Reader) (ImportResult, error) {
	scanner := bufio.NewScanner(r)
	objects := []map[string]interface{}{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			return ImportResult{}, err
		}
		objects = append(objects, obj)
	}
	if err := scanner.Err(); err != nil {
		return ImportResult{}, err
	}
	rows, columns := rowsFromObjects(objects)
	return loadRows(ctx, cfg, tableName, columns, rows)
}

// LoadYAML inserts rows from either output.Result YAML or an array of objects.
func LoadYAML(ctx context.Context, cfg Config, tableName string, r io.Reader) (ImportResult, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return ImportResult{}, err
	}
	var result output.Result
	if err := yaml.Unmarshal(data, &result); err == nil && len(result.Columns) > 0 {
		return loadRows(ctx, cfg, tableName, result.Columns, result.Rows)
	}
	var objects []map[string]interface{}
	if err := yaml.Unmarshal(data, &objects); err != nil {
		return ImportResult{}, err
	}
	rows, columns := rowsFromObjects(objects)
	return loadRows(ctx, cfg, tableName, columns, rows)
}

func rowsFromObjects(objects []map[string]interface{}) ([][]interface{}, []string) {
	columnSet := map[string]bool{}
	for _, obj := range objects {
		for column := range obj {
			columnSet[column] = true
		}
	}
	columns := make([]string, 0, len(columnSet))
	for column := range columnSet {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	rows := make([][]interface{}, len(objects))
	for i, obj := range objects {
		row := make([]interface{}, len(columns))
		for j, column := range columns {
			row[j] = obj[column]
		}
		rows[i] = row
	}
	return rows, columns
}

func loadRows(ctx context.Context, cfg Config, tableName string, columns []string, rows [][]interface{}) (ImportResult, error) {
	conn, driver, err := Open(ctx, cfg)
	if err != nil {
		return ImportResult{}, err
	}
	defer conn.Close()
	if strings.TrimSpace(tableName) == "" {
		return ImportResult{}, fmt.Errorf("table is required")
	}
	if len(columns) == 0 {
		return ImportResult{}, fmt.Errorf("input columns are required")
	}
	quoted := make([]string, len(columns))
	placeholders := make([]string, len(columns))
	for i, column := range columns {
		if strings.TrimSpace(column) == "" {
			return ImportResult{}, fmt.Errorf("input contains empty column")
		}
		quoted[i] = quoteIdent(driver, column)
		placeholders[i] = placeholder(driver, i+1)
	}
	stmtSQL := fmt.Sprintf("insert into %s (%s) values (%s)", quoteIdent(driver, tableName), strings.Join(quoted, ","), strings.Join(placeholders, ","))
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return ImportResult{}, err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, stmtSQL)
	if err != nil {
		return ImportResult{}, err
	}
	defer stmt.Close()
	result := ImportResult{}
	for _, row := range rows {
		if len(row) != len(columns) {
			return ImportResult{}, fmt.Errorf("input row has %d fields, want %d", len(row), len(columns))
		}
		if _, err := stmt.ExecContext(ctx, row...); err != nil {
			return ImportResult{}, err
		}
		result.RowsAffected++
	}
	if err := tx.Commit(); err != nil {
		return ImportResult{}, err
	}
	return result, nil
}

// returnsRows reports whether statement should be executed with QueryContext
// instead of ExecContext based on its first SQL token.
func returnsRows(statement string) bool {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(statement)))
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "select", "with", "show", "describe", "explain", "pragma":
		return true
	default:
		return false
	}
}

func quoteIdent(driver, ident string) string {
	quote := `"`
	if driver == dbdriver.DriverMySQL {
		quote = "`"
	}
	parts := strings.Split(ident, ".")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if quote == "`" {
			part = strings.ReplaceAll(part, "`", "``")
		} else {
			part = strings.ReplaceAll(part, `"`, `""`)
		}
		parts[i] = quote + part + quote
	}
	return strings.Join(parts, ".")
}

func placeholder(driver string, position int) string {
	switch driver {
	case dbdriver.DriverPGX:
		return fmt.Sprintf("$%d", position)
	case dbdriver.DriverSQLServer:
		return fmt.Sprintf("@p%d", position)
	case dbdriver.DriverOracle:
		return fmt.Sprintf(":%d", position)
	default:
		return "?"
	}
}

// normalize maps user-facing driver aliases onto database/sql driver names and
// validates that a DSN is present.
func normalize(cfg Config) (string, string, error) {
	driver, ok := dbdriver.Normalize(cfg.Driver)
	if !ok {
		return "", "", fmt.Errorf("unsupported driver: %s", cfg.Driver)
	}
	switch driver {
	case dbdriver.DriverSQLite:
		if cfg.DSN == "" {
			return "", "", fmt.Errorf("sqlite requires dsn or database path")
		}
		return dbdriver.DriverSQLite, cfg.DSN, nil
	case dbdriver.DriverPGX:
		if cfg.DSN == "" {
			return "", "", fmt.Errorf("postgres requires dsn")
		}
		return dbdriver.DriverPGX, cfg.DSN, nil
	case dbdriver.DriverMySQL:
		if cfg.DSN == "" {
			return "", "", fmt.Errorf("mysql requires dsn")
		}
		return dbdriver.DriverMySQL, cfg.DSN, nil
	case dbdriver.DriverSQLServer:
		if cfg.DSN == "" {
			return "", "", fmt.Errorf("sqlserver requires dsn")
		}
		return dbdriver.DriverSQLServer, cfg.DSN, nil
	case dbdriver.DriverOracle:
		if cfg.DSN == "" {
			return "", "", fmt.Errorf("oracle requires dsn")
		}
		return dbdriver.DriverOracle, cfg.DSN, nil
	case dbdriver.DriverClickHouse:
		if cfg.DSN == "" {
			return "", "", fmt.Errorf("clickhouse requires dsn")
		}
		return dbdriver.DriverClickHouse, cfg.DSN, nil
	case dbdriver.DriverDuckDB:
		if cfg.DSN == "" {
			return "", "", fmt.Errorf("duckdb requires dsn or database path")
		}
		return dbdriver.DriverDuckDB, cfg.DSN, nil
	}
	return "", "", fmt.Errorf("unsupported driver: %s", cfg.Driver)
}

// scanRows copies database rows into a driver-neutral output.Result, converting
// byte slices to strings for readable CLI output.
func scanRows(rows *sql.Rows, maxRows int) (output.Result, error) {
	columns, err := rows.Columns()
	if err != nil {
		return output.Result{}, err
	}
	result := output.Result{Columns: columns}
	values := make([]interface{}, len(columns))
	dest := make([]interface{}, len(columns))
	for i := range values {
		dest[i] = &values[i]
	}
	for rows.Next() {
		if maxRows > 0 && len(result.Rows) >= maxRows {
			break
		}
		for i := range values {
			values[i] = nil
		}
		if err := rows.Scan(dest...); err != nil {
			return output.Result{}, err
		}
		row := make([]interface{}, len(columns))
		for i, value := range values {
			if bytes, ok := value.([]byte); ok {
				row[i] = string(bytes)
			} else {
				row[i] = value
			}
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return output.Result{}, err
	}
	result.RowCount = len(result.Rows)
	return result, nil
}

func streamRows(rows *sql.Rows, maxRows int, w io.Writer, format string, started time.Time) (output.Result, error) {
	columns, err := rows.Columns()
	if err != nil {
		return output.Result{}, err
	}
	values := make([]interface{}, len(columns))
	dest := make([]interface{}, len(columns))
	for i := range values {
		dest[i] = &values[i]
	}
	writer, err := output.NewStreamWriter(w, format, columns, func() int64 { return elapsed(started) })
	if err != nil {
		return output.Result{}, err
	}
	for rows.Next() {
		if maxRows > 0 && writer.RowCount() >= maxRows {
			break
		}
		for i := range values {
			values[i] = nil
		}
		if err := rows.Scan(dest...); err != nil {
			return output.Result{}, err
		}
		row := make([]interface{}, len(columns))
		for i, value := range values {
			if bytes, ok := value.([]byte); ok {
				row[i] = string(bytes)
			} else {
				row[i] = value
			}
		}
		if err := writer.WriteRow(row); err != nil {
			return output.Result{}, err
		}
	}
	if err := rows.Err(); err != nil {
		return output.Result{}, err
	}
	return writer.Close()
}

// explainSQL prefixes each statement with the driver's EXPLAIN syntax.
func explainSQL(driver, sqlText string, analyze bool) string {
	prefix := "EXPLAIN "
	switch {
	case driver == dbdriver.DriverSQLite:
		prefix = "EXPLAIN QUERY PLAN "
	case analyze:
		prefix = "EXPLAIN ANALYZE "
	}
	statements := query.Statements(sqlText)
	for i, statement := range statements {
		statements[i] = prefix + statement
	}
	return strings.Join(statements, ";")
}

// elapsed returns milliseconds since started.
func elapsed(started time.Time) int64 {
	return time.Since(started).Milliseconds()
}
