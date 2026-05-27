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
	Schema  string       `json:"schema,omitempty"`
	Name    string       `json:"name"`
	Columns []ColumnInfo `json:"columns"`
	Indexes []IndexInfo  `json:"indexes,omitempty"`
	DDL     string       `json:"ddl"`
}

// IndexInfo describes one table index.
type IndexInfo struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
	Primary bool     `json:"primary,omitempty"`
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
	conn, driver, err := openConnection(ctx, cfg)
	if err != nil {
		return SchemaInfo{}, err
	}
	defer conn.Close()
	switch driver {
	case "sqlite":
		return sqliteMetadata(ctx, conn)
	case "duckdb":
		return duckDBMetadata(ctx, conn, cfg.Schema)
	case "mysql":
		return mysqlMetadata(ctx, conn, cfg.Schema)
	case "pgx":
		return postgresMetadata(ctx, conn, cfg.Schema)
	case "sqlserver":
		return sqlServerMetadata(ctx, conn, cfg.Schema)
	case "oracle":
		return oracleMetadata(ctx, conn, cfg.Schema)
	case "clickhouse":
		return clickHouseMetadata(ctx, conn, cfg.Schema)
	default:
		return SchemaInfo{}, fmt.Errorf("unsupported metadata driver: %s", cfg.Driver)
	}
}

// Schemas returns schema/database names when the driver exposes them.
func Schemas(ctx context.Context, cfg Config) ([]string, error) {
	conn, driver, err := openConnection(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	switch driver {
	case "sqlite":
		rows, err := conn.QueryContext(ctx, `pragma database_list`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		names := []string{}
		for rows.Next() {
			var seq int
			var name, file string
			if err := rows.Scan(&seq, &name, &file); err != nil {
				return nil, err
			}
			names = append(names, name)
		}
		return names, rows.Err()
	case "duckdb":
		return scanSchemaNames(ctx, conn, `select schema_name from information_schema.schemata order by schema_name`)
	case "mysql":
		return scanSchemaNames(ctx, conn, `select schema_name from information_schema.schemata order by schema_name`)
	case "pgx":
		return scanSchemaNames(ctx, conn, `select nspname from pg_catalog.pg_namespace where nspname not like 'pg_%' and nspname <> 'information_schema' order by nspname`)
	case "sqlserver":
		return scanSchemaNames(ctx, conn, `select schema_name from information_schema.schemata order by schema_name`)
	case "oracle":
		return scanSchemaNames(ctx, conn, `select username from all_users order by username`)
	case "clickhouse":
		return scanSchemaNames(ctx, conn, `select name from system.databases order by name`)
	default:
		return nil, fmt.Errorf("unsupported metadata driver: %s", cfg.Driver)
	}
}

func scanSchemaNames(ctx context.Context, conn *sql.DB, query string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
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
		indexes, err := sqliteIndexes(ctx, conn, table.Name)
		if err != nil {
			return SchemaInfo{}, err
		}
		table.Columns = columns
		table.Indexes = indexes
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
	indexes, err := sqliteIndexes(ctx, conn, tableName)
	if err != nil {
		return nil, err
	}
	uniqueColumns := map[string]bool{}
	for _, index := range indexes {
		if index.Unique && len(index.Columns) == 1 {
			uniqueColumns[index.Columns[0]] = true
		}
	}
	return uniqueColumns, nil
}

// sqliteIndexes reads SQLite index metadata for one table.
func sqliteIndexes(ctx context.Context, conn *sql.DB, tableName string) ([]IndexInfo, error) {
	rows, err := conn.QueryContext(ctx, `pragma index_list(`+quoteSQLiteIdent(tableName)+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	indexes := []IndexInfo{}
	for rows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial interface{}
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return nil, err
		}
		indexColumns, err := sqliteIndexColumns(ctx, conn, name)
		if err != nil {
			return nil, err
		}
		indexes = append(indexes, IndexInfo{Name: name, Columns: indexColumns, Unique: unique != 0, Primary: origin == "pk"})
	}
	return indexes, rows.Err()
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
func mysqlMetadata(ctx context.Context, conn *sql.DB, schemaName string) (SchemaInfo, error) {
	rows, err := conn.QueryContext(ctx, `
select table_name
from information_schema.tables
where table_schema = coalesce(nullif(?, ''), database())
  and table_type in ('BASE TABLE', 'VIEW')
order by table_name`, schemaName)
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
		table.Schema = schemaName
		columns, err := mysqlColumns(ctx, conn, schemaName, table.Name)
		if err != nil {
			return SchemaInfo{}, err
		}
		indexes, err := mysqlIndexes(ctx, conn, schemaName, table.Name)
		if err != nil {
			return SchemaInfo{}, err
		}
		table.Columns = columns
		table.Indexes = indexes
		table.DDL = mysqlDDL(table)
		schema.Tables = append(schema.Tables, table)
	}
	return schema, rows.Err()
}

// mysqlColumns reads MySQL column, key, default, and single-column foreign key
// metadata for tableName.
func mysqlColumns(ctx context.Context, conn *sql.DB, schemaName, tableName string) ([]ColumnInfo, error) {
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
where c.table_schema = coalesce(nullif(?, ''), database())
  and c.table_name = ?
order by ordinal_position`, schemaName, tableName)
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

// mysqlIndexes reads MySQL index metadata for one table.
func mysqlIndexes(ctx context.Context, conn *sql.DB, schemaName, tableName string) ([]IndexInfo, error) {
	rows, err := conn.QueryContext(ctx, `
select index_name,
       column_name,
       non_unique,
       seq_in_index
from information_schema.statistics
where table_schema = coalesce(nullif(?, ''), database())
  and table_name = ?
order by index_name, seq_in_index`, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	indexes := []IndexInfo{}
	byName := map[string]int{}
	for rows.Next() {
		var name, column string
		var nonUnique int
		var seq int
		if err := rows.Scan(&name, &column, &nonUnique, &seq); err != nil {
			return nil, err
		}
		pos, ok := byName[name]
		if !ok {
			indexes = append(indexes, IndexInfo{Name: name, Unique: nonUnique == 0, Primary: name == "PRIMARY"})
			pos = len(indexes) - 1
			byName[name] = pos
		}
		indexes[pos].Columns = append(indexes[pos].Columns, column)
	}
	return indexes, rows.Err()
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
func postgresMetadata(ctx context.Context, conn *sql.DB, schemaName string) (SchemaInfo, error) {
	rows, err := conn.QueryContext(ctx, `
select table_name
from information_schema.tables
where table_schema = coalesce(nullif($1, ''), current_schema())
  and table_type in ('BASE TABLE', 'VIEW')
order by table_name`, schemaName)
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
		table.Schema = schemaName
		columns, err := postgresColumns(ctx, conn, schemaName, table.Name)
		if err != nil {
			return SchemaInfo{}, err
		}
		indexes, err := postgresIndexes(ctx, conn, schemaName, table.Name)
		if err != nil {
			return SchemaInfo{}, err
		}
		table.Columns = columns
		table.Indexes = indexes
		table.DDL = postgresDDL(table)
		schema.Tables = append(schema.Tables, table)
	}
	return schema, rows.Err()
}

// postgresColumns reads PostgreSQL column, default, uniqueness, primary key,
// and single-column foreign key metadata for tableName.
func postgresColumns(ctx context.Context, conn *sql.DB, schemaName, tableName string) ([]ColumnInfo, error) {
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
where n.nspname = coalesce(nullif($1, ''), current_schema())
  and c.relname = $2
  and a.attnum > 0
  and not a.attisdropped
order by a.attnum`, schemaName, tableName)
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

// postgresIndexes reads PostgreSQL index metadata for one table.
func postgresIndexes(ctx context.Context, conn *sql.DB, schemaName, tableName string) ([]IndexInfo, error) {
	rows, err := conn.QueryContext(ctx, `
select ic.relname as index_name,
       array_to_string(array_agg(a.attname order by k.ordinality), ',') as columns,
       i.indisunique,
       i.indisprimary
from pg_catalog.pg_class c
join pg_catalog.pg_namespace n
  on n.oid = c.relnamespace
join pg_catalog.pg_index i
  on i.indrelid = c.oid
join pg_catalog.pg_class ic
  on ic.oid = i.indexrelid
join unnest(i.indkey) with ordinality as k(attnum, ordinality)
  on true
join pg_catalog.pg_attribute a
  on a.attrelid = c.oid
 and a.attnum = k.attnum
where n.nspname = coalesce(nullif($1, ''), current_schema())
  and c.relname = $2
group by ic.relname, i.indisunique, i.indisprimary
order by ic.relname`, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	indexes := []IndexInfo{}
	for rows.Next() {
		var index IndexInfo
		var columns string
		if err := rows.Scan(&index.Name, &columns, &index.Unique, &index.Primary); err != nil {
			return nil, err
		}
		if columns != "" {
			index.Columns = strings.Split(columns, ",")
		}
		indexes = append(indexes, index)
	}
	return indexes, rows.Err()
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

func sqlServerMetadata(ctx context.Context, conn *sql.DB, schemaName string) (SchemaInfo, error) {
	return informationSchemaMetadata(ctx, conn, metadataDialect{
		Schema:        schemaName,
		Param:         "@p1",
		CurrentSchema: "schema_name()",
		TypeExpr:      "coalesce(c.data_type + coalesce('(' + cast(c.character_maximum_length as varchar(20)) + ')', ''), c.data_type)",
		TableFilter:   "c.table_catalog = t.table_catalog and c.table_schema = t.table_schema",
		Quote:         quoteSQLIdent,
	})
}

func oracleMetadata(ctx context.Context, conn *sql.DB, schemaName string) (SchemaInfo, error) {
	rows, err := conn.QueryContext(ctx, `
select table_name
from all_tables
where owner = coalesce(nullif(:1, ''), sys_context('USERENV', 'CURRENT_SCHEMA'))
union
select view_name
from all_views
where owner = coalesce(nullif(:1, ''), sys_context('USERENV', 'CURRENT_SCHEMA'))
order by 1`, schemaName)
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
		table.Schema = schemaName
		columns, err := oracleColumns(ctx, conn, schemaName, table.Name)
		if err != nil {
			return SchemaInfo{}, err
		}
		indexes, err := oracleIndexes(ctx, conn, schemaName, table.Name)
		if err != nil {
			return SchemaInfo{}, err
		}
		table.Columns = columns
		table.Indexes = indexes
		table.DDL = genericDDL(table, quoteSQLIdent)
		schema.Tables = append(schema.Tables, table)
	}
	return schema, rows.Err()
}

func oracleColumns(ctx context.Context, conn *sql.DB, schemaName, tableName string) ([]ColumnInfo, error) {
	rows, err := conn.QueryContext(ctx, `
select column_name,
       data_type,
       nullable,
       data_default
from all_tab_columns
where owner = coalesce(nullif(:1, ''), sys_context('USERENV', 'CURRENT_SCHEMA'))
  and table_name = :2
order by column_id`, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := []ColumnInfo{}
	for rows.Next() {
		var column ColumnInfo
		var nullable string
		var defaultValue sql.NullString
		if err := rows.Scan(&column.Name, &column.Type, &nullable, &defaultValue); err != nil {
			return nil, err
		}
		column.Nullable = nullable == "Y"
		if defaultValue.Valid {
			column.Default = strings.TrimSpace(defaultValue.String)
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func oracleIndexes(ctx context.Context, conn *sql.DB, schemaName, tableName string) ([]IndexInfo, error) {
	rows, err := conn.QueryContext(ctx, `
select i.index_name,
       c.column_name,
       i.uniqueness,
       c.column_position
from all_indexes i
join all_ind_columns c
  on c.index_owner = i.owner
 and c.index_name = i.index_name
where i.table_owner = coalesce(nullif(:1, ''), sys_context('USERENV', 'CURRENT_SCHEMA'))
  and i.table_name = :2
order by i.index_name, c.column_position`, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	indexes := []IndexInfo{}
	byName := map[string]int{}
	for rows.Next() {
		var name, column, uniqueness string
		var position int
		if err := rows.Scan(&name, &column, &uniqueness, &position); err != nil {
			return nil, err
		}
		idx, ok := byName[name]
		if !ok {
			indexes = append(indexes, IndexInfo{Name: name, Unique: uniqueness == "UNIQUE"})
			idx = len(indexes) - 1
			byName[name] = idx
		}
		indexes[idx].Columns = append(indexes[idx].Columns, column)
	}
	return indexes, rows.Err()
}

func clickHouseMetadata(ctx context.Context, conn *sql.DB, schemaName string) (SchemaInfo, error) {
	rows, err := conn.QueryContext(ctx, `
select name
from system.tables
where database = if({schema:String} = '', currentDatabase(), {schema:String})
order by name`, sql.Named("schema", schemaName))
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
		table.Schema = schemaName
		columns, err := clickHouseColumns(ctx, conn, schemaName, table.Name)
		if err != nil {
			return SchemaInfo{}, err
		}
		table.Columns = columns
		table.DDL = genericDDL(table, quoteSQLIdent)
		schema.Tables = append(schema.Tables, table)
	}
	return schema, rows.Err()
}

func duckDBMetadata(ctx context.Context, conn *sql.DB, schemaName string) (SchemaInfo, error) {
	return informationSchemaMetadata(ctx, conn, metadataDialect{
		Schema:        schemaName,
		Param:         "?",
		CurrentSchema: "current_schema()",
		TypeExpr:      "c.data_type",
		TableFilter:   "c.table_schema = t.table_schema",
		TableParam:    "?",
		Quote:         quoteSQLIdent,
	})
}

func clickHouseColumns(ctx context.Context, conn *sql.DB, schemaName, tableName string) ([]ColumnInfo, error) {
	rows, err := conn.QueryContext(ctx, `
select name,
       type,
       default_expression
from system.columns
where database = if({schema:String} = '', currentDatabase(), {schema:String})
  and table = {table:String}
order by position`, sql.Named("schema", schemaName), sql.Named("table", tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := []ColumnInfo{}
	for rows.Next() {
		var column ColumnInfo
		var defaultValue sql.NullString
		if err := rows.Scan(&column.Name, &column.Type, &defaultValue); err != nil {
			return nil, err
		}
		column.Nullable = strings.HasPrefix(strings.ToLower(column.Type), "nullable(")
		if defaultValue.Valid {
			column.Default = defaultValue.String
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

type metadataDialect struct {
	Schema        string
	Param         string
	CurrentSchema string
	TypeExpr      string
	TableFilter   string
	TableParam    string
	Quote         func(string) string
}

func informationSchemaMetadata(ctx context.Context, conn *sql.DB, d metadataDialect) (SchemaInfo, error) {
	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`
select table_name
from information_schema.tables
where table_schema = coalesce(nullif(%s, ''), %s)
  and table_type in ('BASE TABLE', 'VIEW')
order by table_name`, d.Param, d.CurrentSchema), d.Schema)
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
		table.Schema = d.Schema
		columns, err := informationSchemaColumns(ctx, conn, d, table.Name)
		if err != nil {
			return SchemaInfo{}, err
		}
		table.Columns = columns
		table.DDL = genericDDL(table, d.Quote)
		schema.Tables = append(schema.Tables, table)
	}
	return schema, rows.Err()
}

func informationSchemaColumns(ctx context.Context, conn *sql.DB, d metadataDialect, tableName string) ([]ColumnInfo, error) {
	tableParam := d.TableParam
	if tableParam == "" {
		tableParam = "@p2"
	}
	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`
select c.column_name,
       %s as column_type,
       c.is_nullable,
       c.column_default
from information_schema.columns c
join information_schema.tables t
  on %s
 and c.table_name = t.table_name
where c.table_schema = coalesce(nullif(%s, ''), %s)
  and c.table_name = %s
order by c.ordinal_position`, d.TypeExpr, d.TableFilter, d.Param, d.CurrentSchema, tableParam), d.Schema, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := []ColumnInfo{}
	for rows.Next() {
		var column ColumnInfo
		var nullable string
		var defaultValue sql.NullString
		if err := rows.Scan(&column.Name, &column.Type, &nullable, &defaultValue); err != nil {
			return nil, err
		}
		column.Nullable = nullable == "YES"
		if defaultValue.Valid {
			column.Default = defaultValue.String
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func genericDDL(table TableInfo, quote func(string) string) string {
	parts := make([]string, 0, len(table.Columns))
	for _, column := range table.Columns {
		part := fmt.Sprintf("%s %s", quote(column.Name), column.Type)
		if !column.Nullable {
			part += " NOT NULL"
		}
		if column.Default != "" {
			part += " DEFAULT " + column.Default
		}
		parts = append(parts, part)
	}
	return fmt.Sprintf("CREATE TABLE %s (%s);", quote(table.Name), strings.Join(parts, ", "))
}

func quoteSQLIdent(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
