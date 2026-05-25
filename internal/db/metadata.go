package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type SchemaInfo struct {
	Tables []TableInfo `json:"tables"`
}

type TableInfo struct {
	Name    string       `json:"name"`
	Columns []ColumnInfo `json:"columns"`
	DDL     string       `json:"ddl"`
}

type ColumnInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Primary  bool   `json:"primary"`
}

func Metadata(ctx context.Context, cfg Config) (SchemaInfo, error) {
	conn, driver, err := Open(ctx, cfg)
	if err != nil {
		return SchemaInfo{}, err
	}
	defer conn.Close()
	switch driver {
	case "sqlite":
		return sqliteMetadata(ctx, conn)
	case "mysql":
		return mysqlMetadata(ctx, conn)
	case "pgx":
		return postgresMetadata(ctx, conn)
	default:
		return SchemaInfo{}, fmt.Errorf("unsupported metadata driver: %s", cfg.Driver)
	}
}

func sqliteMetadata(ctx context.Context, conn *sql.DB) (SchemaInfo, error) {
	rows, err := conn.QueryContext(ctx, `select name, sql from sqlite_master where type in ('table', 'view') and name not like 'sqlite_%' order by name`)
	if err != nil {
		return SchemaInfo{}, err
	}
	defer rows.Close()

	schema := SchemaInfo{}
	for rows.Next() {
		var table TableInfo
		if err := rows.Scan(&table.Name, &table.DDL); err != nil {
			return SchemaInfo{}, err
		}
		columns, err := sqliteColumns(ctx, conn, table.Name)
		if err != nil {
			return SchemaInfo{}, err
		}
		table.Columns = columns
		schema.Tables = append(schema.Tables, table)
	}
	if err := rows.Err(); err != nil {
		return SchemaInfo{}, err
	}
	return schema, nil
}

func sqliteColumns(ctx context.Context, conn *sql.DB, tableName string) ([]ColumnInfo, error) {
	rows, err := conn.QueryContext(ctx, `pragma table_info(`+quoteSQLiteIdent(tableName)+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := []ColumnInfo{}
	for rows.Next() {
		var cid int
		var column ColumnInfo
		var notNull int
		var defaultValue interface{}
		var pk int
		if err := rows.Scan(&cid, &column.Name, &column.Type, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		column.Nullable = notNull == 0 && pk == 0
		column.Primary = pk > 0
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func quoteSQLiteIdent(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func mysqlMetadata(ctx context.Context, conn *sql.DB) (SchemaInfo, error) {
	rows, err := conn.QueryContext(ctx, `
select table_name
from information_schema.tables
where table_schema = database()
  and table_type in ('BASE TABLE', 'VIEW')
order by table_name`)
	if err != nil {
		return SchemaInfo{}, err
	}
	defer rows.Close()

	schema := SchemaInfo{}
	for rows.Next() {
		var table TableInfo
		if err := rows.Scan(&table.Name); err != nil {
			return SchemaInfo{}, err
		}
		columns, err := mysqlColumns(ctx, conn, table.Name)
		if err != nil {
			return SchemaInfo{}, err
		}
		table.Columns = columns
		table.DDL = mysqlDDL(table)
		schema.Tables = append(schema.Tables, table)
	}
	return schema, rows.Err()
}

func mysqlColumns(ctx context.Context, conn *sql.DB, tableName string) ([]ColumnInfo, error) {
	rows, err := conn.QueryContext(ctx, `
select column_name, column_type, is_nullable, column_key
from information_schema.columns
where table_schema = database()
  and table_name = ?
order by ordinal_position`, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := []ColumnInfo{}
	for rows.Next() {
		var column ColumnInfo
		var nullable string
		var key string
		if err := rows.Scan(&column.Name, &column.Type, &nullable, &key); err != nil {
			return nil, err
		}
		column.Nullable = nullable == "YES"
		column.Primary = key == "PRI"
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func mysqlDDL(table TableInfo) string {
	parts := make([]string, 0, len(table.Columns))
	for _, column := range table.Columns {
		part := fmt.Sprintf("`%s` %s", strings.ReplaceAll(column.Name, "`", "``"), column.Type)
		if !column.Nullable {
			part += " NOT NULL"
		}
		if column.Primary {
			part += " PRIMARY KEY"
		}
		parts = append(parts, part)
	}
	return fmt.Sprintf("CREATE TABLE `%s` (%s);", strings.ReplaceAll(table.Name, "`", "``"), strings.Join(parts, ", "))
}

func postgresMetadata(ctx context.Context, conn *sql.DB) (SchemaInfo, error) {
	rows, err := conn.QueryContext(ctx, `
select table_name
from information_schema.tables
where table_schema = current_schema()
  and table_type in ('BASE TABLE', 'VIEW')
order by table_name`)
	if err != nil {
		return SchemaInfo{}, err
	}
	defer rows.Close()

	schema := SchemaInfo{}
	for rows.Next() {
		var table TableInfo
		if err := rows.Scan(&table.Name); err != nil {
			return SchemaInfo{}, err
		}
		columns, err := postgresColumns(ctx, conn, table.Name)
		if err != nil {
			return SchemaInfo{}, err
		}
		table.Columns = columns
		table.DDL = postgresDDL(table)
		schema.Tables = append(schema.Tables, table)
	}
	return schema, rows.Err()
}

func postgresColumns(ctx context.Context, conn *sql.DB, tableName string) ([]ColumnInfo, error) {
	rows, err := conn.QueryContext(ctx, `
select c.column_name,
       coalesce(c.udt_name, c.data_type) as column_type,
       c.is_nullable,
       case when tc.constraint_type = 'PRIMARY KEY' then true else false end as is_primary
from information_schema.columns c
left join information_schema.key_column_usage kcu
  on c.table_schema = kcu.table_schema
 and c.table_name = kcu.table_name
 and c.column_name = kcu.column_name
left join information_schema.table_constraints tc
  on kcu.constraint_schema = tc.constraint_schema
 and kcu.constraint_name = tc.constraint_name
 and tc.constraint_type = 'PRIMARY KEY'
where c.table_schema = current_schema()
  and c.table_name = $1
order by c.ordinal_position`, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := []ColumnInfo{}
	for rows.Next() {
		var column ColumnInfo
		var nullable string
		if err := rows.Scan(&column.Name, &column.Type, &nullable, &column.Primary); err != nil {
			return nil, err
		}
		column.Nullable = nullable == "YES"
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func postgresDDL(table TableInfo) string {
	parts := make([]string, 0, len(table.Columns))
	for _, column := range table.Columns {
		part := fmt.Sprintf(`"%s" %s`, strings.ReplaceAll(column.Name, `"`, `""`), column.Type)
		if !column.Nullable {
			part += " NOT NULL"
		}
		if column.Primary {
			part += " PRIMARY KEY"
		}
		parts = append(parts, part)
	}
	return fmt.Sprintf(`CREATE TABLE "%s" (%s);`, strings.ReplaceAll(table.Name, `"`, `""`), strings.Join(parts, ", "))
}
