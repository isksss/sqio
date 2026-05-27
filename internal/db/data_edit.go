package db

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// InsertRow inserts one row into tableName.
func InsertRow(ctx context.Context, cfg Config, tableName string, values map[string]string) (int, error) {
	conn, driver, err := openConnection(ctx, cfg)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	if strings.TrimSpace(tableName) == "" {
		return 0, fmt.Errorf("table is required")
	}
	if len(values) == 0 {
		return 0, fmt.Errorf("at least one value is required")
	}
	columns := sortedKeys(values)
	quoted := make([]string, len(columns))
	placeholders := make([]string, len(columns))
	args := make([]interface{}, len(columns))
	for i, column := range columns {
		quoted[i] = quoteIdent(driver, column)
		placeholders[i] = placeholder(driver, i+1)
		args[i] = values[column]
	}
	stmt := fmt.Sprintf("insert into %s (%s) values (%s)", quoteIdent(driver, tableName), strings.Join(quoted, ","), strings.Join(placeholders, ","))
	result, err := conn.ExecContext(ctx, stmt, args...)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// UpdateRows updates rows matching whereClause.
func UpdateRows(ctx context.Context, cfg Config, tableName string, values map[string]string, whereClause string) (int, error) {
	conn, driver, err := openConnection(ctx, cfg)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	if strings.TrimSpace(tableName) == "" {
		return 0, fmt.Errorf("table is required")
	}
	if len(values) == 0 {
		return 0, fmt.Errorf("at least one value is required")
	}
	if strings.TrimSpace(whereClause) == "" {
		return 0, fmt.Errorf("where clause is required")
	}
	columns := sortedKeys(values)
	assignments := make([]string, len(columns))
	args := make([]interface{}, len(columns))
	for i, column := range columns {
		assignments[i] = quoteIdent(driver, column) + " = " + placeholder(driver, i+1)
		args[i] = values[column]
	}
	stmt := fmt.Sprintf("update %s set %s where %s", quoteIdent(driver, tableName), strings.Join(assignments, ","), whereClause)
	result, err := conn.ExecContext(ctx, stmt, args...)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// DeleteRows deletes rows matching whereClause.
func DeleteRows(ctx context.Context, cfg Config, tableName, whereClause string) (int, error) {
	conn, driver, err := openConnection(ctx, cfg)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	if strings.TrimSpace(tableName) == "" {
		return 0, fmt.Errorf("table is required")
	}
	if strings.TrimSpace(whereClause) == "" {
		return 0, fmt.Errorf("where clause is required")
	}
	stmt := fmt.Sprintf("delete from %s where %s", quoteIdent(driver, tableName), whereClause)
	result, err := conn.ExecContext(ctx, stmt)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
