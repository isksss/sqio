package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// SchemaInfo is database metadata collected from a live connection.
type SchemaInfo struct {
	Tables []TableInfo `json:"tables"`
}

// TableInfo describes one table or view and its reconstructed DDL when
// available.
type TableInfo struct {
	Name    string       `json:"name"`
	Columns []ColumnInfo `json:"columns"`
	DDL     string       `json:"ddl"`
}

// ColumnInfo describes one column and common relational constraints.
type ColumnInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Nullable   bool   `json:"nullable"`
	Primary    bool   `json:"primary"`
	Unique     bool   `json:"unique,omitempty"`
	Default    string `json:"default,omitempty"`
	References string `json:"references,omitempty"`
}

// Metadata opens cfg and dispatches to the dialect-specific metadata reader.
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

// sqliteMetadata reads SQLite table and view definitions from sqlite_master and
// augments them with PRAGMA-based column metadata.
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

// sqliteColumns reads SQLite column metadata and merges uniqueness and foreign
// key information from separate PRAGMA calls.
func sqliteColumns(ctx context.Context, conn *sql.DB, tableName string) ([]ColumnInfo, error) {
	rows, err := conn.QueryContext(ctx, `pragma table_info(`+quoteSQLiteIdent(tableName)+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := []ColumnInfo{}
	foreignKeys, err := sqliteForeignKeys(ctx, conn, tableName)
	if err != nil {
		return nil, err
	}
	uniqueColumns, err := sqliteUniqueColumns(ctx, conn, tableName)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var cid int
		var column ColumnInfo
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &column.Name, &column.Type, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		column.Nullable = notNull == 0 && pk == 0
		column.Primary = pk > 0
		column.Unique = uniqueColumns[column.Name]
		column.References = foreignKeys[column.Name]
		if defaultValue.Valid {
			column.Default = defaultValue.String
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

// sqliteForeignKeys returns single-column foreign key references keyed by local
// column name.
func sqliteForeignKeys(ctx context.Context, conn *sql.DB, tableName string) (map[string]string, error) {
	rows, err := conn.QueryContext(ctx, `pragma foreign_key_list(`+quoteSQLiteIdent(tableName)+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	foreignKeys := map[string]string{}
	for rows.Next() {
		var id, seq int
		var refTable, from, to, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &refTable, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return nil, err
		}
		foreignKeys[from] = quoteSQLiteIdent(refTable) + "(" + quoteSQLiteIdent(to) + ")"
	}
	return foreignKeys, rows.Err()
}

// sqliteUniqueColumns returns columns covered by a single-column unique index.
func sqliteUniqueColumns(ctx context.Context, conn *sql.DB, tableName string) (map[string]bool, error) {
	rows, err := conn.QueryContext(ctx, `pragma index_list(`+quoteSQLiteIdent(tableName)+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	uniqueColumns := map[string]bool{}
	for rows.Next() {
		var seq int
		var name string
		var unique int
		var origin, partial interface{}
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return nil, err
		}
		if unique == 0 {
			continue
		}
		indexColumns, err := sqliteIndexColumns(ctx, conn, name)
		if err != nil {
			return nil, err
		}
		if len(indexColumns) == 1 {
			uniqueColumns[indexColumns[0]] = true
		}
	}
	return uniqueColumns, rows.Err()
}

// sqliteIndexColumns returns the ordered column names for a SQLite index.
func sqliteIndexColumns(ctx context.Context, conn *sql.DB, indexName string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, `pragma index_info(`+quoteSQLiteIdent(indexName)+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := []string{}
	for rows.Next() {
		var seqno, cid int
		var name string
		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	return columns, rows.Err()
}

// quoteSQLiteIdent quotes a SQLite identifier for use in PRAGMA statements.
func quoteSQLiteIdent(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

// mysqlMetadata reads tables from the current MySQL database and reconstructs
// portable DDL from information_schema column metadata.
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

// mysqlColumns reads MySQL column, key, default, and single-column foreign key
// metadata for tableName.
func mysqlColumns(ctx context.Context, conn *sql.DB, tableName string) ([]ColumnInfo, error) {
	rows, err := conn.QueryContext(ctx, `
select c.column_name,
       c.column_type,
       c.is_nullable,
       c.column_key,
       c.column_default,
       (
         select concat(k.referenced_table_name, '(', k.referenced_column_name, ')')
         from information_schema.key_column_usage k
         where k.table_schema = c.table_schema
           and k.table_name = c.table_name
           and k.column_name = c.column_name
           and k.referenced_table_name is not null
         limit 1
       ) as foreign_ref
from information_schema.columns c
where c.table_schema = database()
  and c.table_name = ?
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
		var defaultValue sql.NullString
		var foreignRef sql.NullString
		if err := rows.Scan(&column.Name, &column.Type, &nullable, &key, &defaultValue, &foreignRef); err != nil {
			return nil, err
		}
		column.Nullable = nullable == "YES"
		column.Primary = key == "PRI"
		column.Unique = key == "UNI"
		if defaultValue.Valid {
			column.Default = defaultValue.String
		}
		if foreignRef.Valid {
			column.References = foreignRef.String
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

// mysqlDDL renders a compact MySQL CREATE TABLE statement from collected
// metadata.
func mysqlDDL(table TableInfo) string {
	parts := make([]string, 0, len(table.Columns))
	for _, column := range table.Columns {
		part := fmt.Sprintf("`%s` %s", strings.ReplaceAll(column.Name, "`", "``"), column.Type)
		if !column.Nullable {
			part += " NOT NULL"
		}
		if column.Default != "" {
			part += " DEFAULT " + column.Default
		}
		if column.Primary {
			part += " PRIMARY KEY"
		}
		if column.Unique {
			part += " UNIQUE"
		}
		if column.References != "" {
			part += " REFERENCES " + column.References
		}
		parts = append(parts, part)
	}
	return fmt.Sprintf("CREATE TABLE `%s` (%s);", strings.ReplaceAll(table.Name, "`", "``"), strings.Join(parts, ", "))
}

// postgresMetadata reads tables from the current PostgreSQL schema and builds
// table metadata from pg_catalog.
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

// postgresColumns reads PostgreSQL column, default, uniqueness, primary key,
// and single-column foreign key metadata for tableName.
func postgresColumns(ctx context.Context, conn *sql.DB, tableName string) ([]ColumnInfo, error) {
	rows, err := conn.QueryContext(ctx, `
select a.attname,
       pg_catalog.format_type(a.atttypid, a.atttypmod) as column_type,
       not a.attnotnull as is_nullable,
       coalesce(i.indisprimary, false) as is_primary,
       exists (
         select 1
         from pg_catalog.pg_index ui
         where ui.indrelid = c.oid
           and ui.indisunique
           and not ui.indisprimary
           and ui.indnatts = 1
           and a.attnum = any(ui.indkey)
       ) as is_unique,
       pg_get_expr(ad.adbin, ad.adrelid) as column_default,
       (
         select quote_ident(fn.nspname) || '.' || quote_ident(fc.relname) || '(' || quote_ident(fa.attname) || ')'
         from pg_catalog.pg_constraint fk
         join pg_catalog.pg_class fc
           on fc.oid = fk.confrelid
         join pg_catalog.pg_namespace fn
           on fn.oid = fc.relnamespace
         join pg_catalog.pg_attribute fa
           on fa.attrelid = fk.confrelid
          and fa.attnum = fk.confkey[1]
         where fk.conrelid = c.oid
           and fk.contype = 'f'
           and array_length(fk.conkey, 1) = 1
           and fk.conkey[1] = a.attnum
         limit 1
       ) as foreign_ref
from pg_catalog.pg_attribute a
join pg_catalog.pg_class c
  on c.oid = a.attrelid
join pg_catalog.pg_namespace n
  on n.oid = c.relnamespace
left join pg_catalog.pg_index i
  on i.indrelid = c.oid
 and i.indisprimary
 and a.attnum = any(i.indkey)
left join pg_catalog.pg_attrdef ad
  on ad.adrelid = a.attrelid
 and ad.adnum = a.attnum
where n.nspname = current_schema()
  and c.relname = $1
  and a.attnum > 0
  and not a.attisdropped
order by a.attnum`, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := []ColumnInfo{}
	for rows.Next() {
		var column ColumnInfo
		var defaultValue sql.NullString
		var foreignRef sql.NullString
		if err := rows.Scan(&column.Name, &column.Type, &column.Nullable, &column.Primary, &column.Unique, &defaultValue, &foreignRef); err != nil {
			return nil, err
		}
		if defaultValue.Valid {
			column.Default = defaultValue.String
		}
		if foreignRef.Valid {
			column.References = foreignRef.String
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

// postgresDDL renders a compact PostgreSQL CREATE TABLE statement from
// collected metadata.
func postgresDDL(table TableInfo) string {
	parts := make([]string, 0, len(table.Columns))
	for _, column := range table.Columns {
		part := fmt.Sprintf(`"%s" %s`, strings.ReplaceAll(column.Name, `"`, `""`), column.Type)
		if !column.Nullable {
			part += " NOT NULL"
		}
		if column.Default != "" {
			part += " DEFAULT " + column.Default
		}
		if column.Primary {
			part += " PRIMARY KEY"
		}
		if column.Unique {
			part += " UNIQUE"
		}
		if column.References != "" {
			part += " REFERENCES " + column.References
		}
		parts = append(parts, part)
	}
	return fmt.Sprintf(`CREATE TABLE "%s" (%s);`, strings.ReplaceAll(table.Name, `"`, `""`), strings.Join(parts, ", "))
}
