package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"testing"
)

func init() {
	sql.Register("sqio_meta_test", metaDriver{})
}

type metaDriver struct{}

func (metaDriver) Open(name string) (driver.Conn, error) {
	return metaConn{name: name}, nil
}

type metaConn struct {
	name string
}

func (c metaConn) Prepare(query string) (driver.Stmt, error) {
	return metaStmt{conn: c, query: query}, nil
}

func (metaConn) Close() error { return nil }

func (metaConn) Begin() (driver.Tx, error) { return metaTx{}, nil }

func (c metaConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return metaRowsFor(c.name, query, args), nil
}

type metaStmt struct {
	conn  metaConn
	query string
}

func (s metaStmt) Close() error  { return nil }
func (s metaStmt) NumInput() int { return -1 }

func (s metaStmt) Exec(args []driver.Value) (driver.Result, error) {
	return driver.RowsAffected(0), nil
}

func (s metaStmt) Query(args []driver.Value) (driver.Rows, error) {
	named := make([]driver.NamedValue, len(args))
	for i, arg := range args {
		named[i] = driver.NamedValue{Ordinal: i + 1, Value: arg}
	}
	return metaRowsFor(s.conn.name, s.query, named), nil
}

type metaTx struct{}

func (metaTx) Commit() error   { return nil }
func (metaTx) Rollback() error { return nil }

type metaRows struct {
	cols []string
	rows [][]driver.Value
	pos  int
}

func (r *metaRows) Columns() []string {
	return r.cols
}

func (r *metaRows) Close() error {
	return nil
}

func (r *metaRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.pos])
	r.pos++
	return nil
}

func metaRowsFor(name, query string, args []driver.NamedValue) driver.Rows {
	normalized := strings.ToLower(query)
	switch {
	case name == "sqlserver" && strings.Contains(normalized, "information_schema.columns"):
		return &metaRows{cols: []string{"column_name", "column_type", "is_nullable", "column_default"}, rows: [][]driver.Value{
			{"id", "int", "NO", nil},
			{"email", "nvarchar(255)", "YES", "''"},
		}}
	case name == "duckdb" && strings.Contains(normalized, "information_schema.columns"):
		return &metaRows{cols: []string{"column_name", "column_type", "is_nullable", "column_default"}, rows: [][]driver.Value{
			{"id", "BIGINT", "NO", nil},
			{"payload", "VARCHAR", "YES", nil},
		}}
	case strings.Contains(normalized, "information_schema.tables"):
		return &metaRows{cols: []string{"table_name"}, rows: [][]driver.Value{{"users"}}}
	case strings.Contains(normalized, "information_schema.schemata"):
		return &metaRows{cols: []string{"schema_name"}, rows: [][]driver.Value{{"app"}, {"analytics"}}}
	case name == "mysql" && strings.Contains(normalized, "information_schema.columns"):
		return &metaRows{cols: []string{"column_name", "column_type", "is_nullable", "column_key", "column_default", "foreign_ref"}, rows: [][]driver.Value{
			{"id", "int", "NO", "PRI", nil, nil},
			{"email", "varchar(255)", "YES", "UNI", "''", nil},
			{"role_id", "int", "YES", "", nil, "roles(id)"},
		}}
	case name == "mysql" && strings.Contains(normalized, "information_schema.statistics"):
		return &metaRows{cols: []string{"index_name", "column_name", "non_unique", "seq_in_index"}, rows: [][]driver.Value{
			{"PRIMARY", "id", int64(0), int64(1)},
			{"users_email_idx", "email", int64(0), int64(1)},
		}}
	case name == "postgres" && strings.Contains(normalized, "array_to_string"):
		return &metaRows{cols: []string{"index_name", "columns", "indisunique", "indisprimary"}, rows: [][]driver.Value{
			{"users_pkey", "id", true, true},
			{"users_email_idx", "email", true, false},
		}}
	case name == "postgres" && strings.Contains(normalized, "pg_catalog.pg_attribute"):
		return &metaRows{cols: []string{"attname", "column_type", "is_nullable", "is_primary", "is_unique", "column_default", "foreign_ref"}, rows: [][]driver.Value{
			{"id", "integer", false, true, false, nil, nil},
			{"email", "character varying(255)", true, false, true, "''::character varying", nil},
			{"role_id", "integer", true, false, false, nil, "public.roles(id)"},
		}}
	case strings.Contains(normalized, "pg_catalog.pg_namespace"):
		return &metaRows{cols: []string{"nspname"}, rows: [][]driver.Value{{"public"}, {"app"}}}
	case strings.Contains(normalized, "pg_catalog.pg_roles"):
		return &metaRows{cols: []string{"rolname", "rolcanlogin", "rolsuper", "rolcreaterole", "rolcreatedb"}, rows: [][]driver.Value{
			{"app_user", true, false, false, false},
			{"admin", true, true, true, true},
		}}
	case strings.Contains(normalized, "information_schema.role_table_grants"):
		rows := [][]driver.Value{
			{"app_user", "public.users", "SELECT", true},
			{"admin", "public.users", "INSERT", false},
		}
		if len(args) > 0 {
			rows = [][]driver.Value{{args[0].Value, "public.users", "SELECT", true}}
		}
		return &metaRows{cols: []string{"grantee", "object", "privilege_type", "is_grantable"}, rows: rows}
	case strings.Contains(normalized, "current_user()"):
		return &metaRows{cols: []string{"current_user"}, rows: [][]driver.Value{{"'app'@'localhost'"}}}
	case strings.Contains(normalized, "show grants"):
		return &metaRows{cols: []string{"grants"}, rows: [][]driver.Value{
			{"GRANT SELECT ON app.* TO 'app'@'localhost'"},
			{"GRANT INSERT ON app.users TO 'app'@'localhost'"},
		}}
	case name == "oracle" && (strings.Contains(normalized, "from all_tables") || strings.Contains(normalized, "from all_views")):
		return &metaRows{cols: []string{"table_name"}, rows: [][]driver.Value{{"USERS"}}}
	case name == "oracle" && strings.Contains(normalized, "from all_users"):
		return &metaRows{cols: []string{"username"}, rows: [][]driver.Value{{"APP"}, {"ANALYTICS"}}}
	case name == "oracle" && strings.Contains(normalized, "all_tab_columns"):
		return &metaRows{cols: []string{"column_name", "data_type", "nullable", "data_default"}, rows: [][]driver.Value{
			{"ID", "NUMBER", "N", nil},
			{"EMAIL", "VARCHAR2(255)", "Y", nil},
		}}
	case name == "oracle" && strings.Contains(normalized, "all_indexes"):
		return &metaRows{cols: []string{"index_name", "column_name", "uniqueness", "column_position"}, rows: [][]driver.Value{
			{"USERS_EMAIL_IDX", "EMAIL", "UNIQUE", int64(1)},
		}}
	case name == "clickhouse" && strings.Contains(normalized, "system.tables"):
		return &metaRows{cols: []string{"name"}, rows: [][]driver.Value{{"events"}}}
	case name == "clickhouse" && strings.Contains(normalized, "system.databases"):
		return &metaRows{cols: []string{"name"}, rows: [][]driver.Value{{"default"}, {"analytics"}}}
	case name == "clickhouse" && strings.Contains(normalized, "system.columns"):
		return &metaRows{cols: []string{"name", "type", "default_expression"}, rows: [][]driver.Value{
			{"id", "UInt64", nil},
			{"payload", "Nullable(String)", nil},
		}}
	default:
		return &metaRows{cols: []string{}, rows: nil}
	}
}

func TestMySQLMetadataWithTestDriver(t *testing.T) {
	conn, err := sql.Open("sqio_meta_test", "mysql")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	schema, err := mysqlMetadata(context.Background(), conn, "app")
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Tables) != 1 || len(schema.Tables[0].Columns) != 3 || len(schema.Tables[0].Indexes) != 2 || schema.Tables[0].Schema != "app" {
		t.Fatalf("unexpected schema: %+v", schema)
	}
	if !strings.Contains(schema.Tables[0].DDL, "`email` varchar(255)") {
		t.Fatalf("unexpected ddl: %s", schema.Tables[0].DDL)
	}
}

func TestPostgresMetadataWithTestDriver(t *testing.T) {
	conn, err := sql.Open("sqio_meta_test", "postgres")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	schema, err := postgresMetadata(context.Background(), conn, "public")
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Tables) != 1 || len(schema.Tables[0].Columns) != 3 || len(schema.Tables[0].Indexes) != 2 || schema.Tables[0].Schema != "public" {
		t.Fatalf("unexpected schema: %+v", schema)
	}
	if !strings.Contains(schema.Tables[0].DDL, `"email" character varying(255)`) {
		t.Fatalf("unexpected ddl: %s", schema.Tables[0].DDL)
	}
}

func TestAdditionalDriverMetadataWithTestDriver(t *testing.T) {
	tests := []struct {
		name     string
		read     func(context.Context, *sql.DB, string) (SchemaInfo, error)
		wantDDL  string
		wantCols int
	}{
		{name: "sqlserver", read: sqlServerMetadata, wantDDL: `"email" nvarchar(255)`, wantCols: 2},
		{name: "oracle", read: oracleMetadata, wantDDL: `"EMAIL" VARCHAR2(255)`, wantCols: 2},
		{name: "clickhouse", read: clickHouseMetadata, wantDDL: `"payload" Nullable(String)`, wantCols: 2},
		{name: "duckdb", read: duckDBMetadata, wantDDL: `"payload" VARCHAR`, wantCols: 2},
	}
	for _, tt := range tests {
		conn, err := sql.Open("sqio_meta_test", tt.name)
		if err != nil {
			t.Fatal(err)
		}
		schema, err := tt.read(context.Background(), conn, "app")
		_ = conn.Close()
		if err != nil {
			t.Fatalf("%s metadata: %v", tt.name, err)
		}
		if len(schema.Tables) != 1 || len(schema.Tables[0].Columns) != tt.wantCols {
			t.Fatalf("%s unexpected schema: %+v", tt.name, schema)
		}
		if !strings.Contains(schema.Tables[0].DDL, tt.wantDDL) {
			t.Fatalf("%s unexpected ddl: %s", tt.name, schema.Tables[0].DDL)
		}
	}
}

func TestScanSchemaNamesWithTestDriver(t *testing.T) {
	conn, err := sql.Open("sqio_meta_test", "mysql")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	names, err := scanSchemaNames(context.Background(), conn, `select schema_name from information_schema.schemata`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(names, ",") != "app,analytics" {
		t.Fatalf("unexpected schema names: %+v", names)
	}
	conn, err = sql.Open("sqio_meta_test", "postgres")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	names, err = scanSchemaNames(context.Background(), conn, `select nspname from pg_catalog.pg_namespace`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(names, ",") != "public,app" {
		t.Fatalf("unexpected pg schema names: %+v", names)
	}
}

func TestMetadataDispatchWithTestDriver(t *testing.T) {
	oldOpen := openConnection
	t.Cleanup(func() { openConnection = oldOpen })
	openConnection = func(_ context.Context, cfg Config) (*sql.DB, string, error) {
		driverName := cfg.Driver
		if driverName == "pgx" {
			driverName = "postgres"
		}
		conn, err := sql.Open("sqio_meta_test", driverName)
		return conn, cfg.Driver, err
	}
	tests := []struct {
		driver string
		want   string
	}{
		{driver: "mysql", want: "users"},
		{driver: "pgx", want: "users"},
		{driver: "sqlserver", want: "users"},
		{driver: "oracle", want: "USERS"},
		{driver: "clickhouse", want: "events"},
		{driver: "duckdb", want: "users"},
	}
	for _, tt := range tests {
		schema, err := Metadata(context.Background(), Config{Driver: tt.driver, Schema: "app"})
		if err != nil {
			t.Fatalf("%s metadata: %v", tt.driver, err)
		}
		if len(schema.Tables) != 1 || schema.Tables[0].Name != tt.want {
			t.Fatalf("%s unexpected schema: %+v", tt.driver, schema)
		}
	}
	if _, err := Metadata(context.Background(), Config{Driver: "unsupported"}); err == nil {
		t.Fatal("expected unsupported metadata driver error")
	}
}

func TestSchemasDispatchWithTestDriver(t *testing.T) {
	oldOpen := openConnection
	t.Cleanup(func() { openConnection = oldOpen })
	openConnection = func(_ context.Context, cfg Config) (*sql.DB, string, error) {
		driverName := cfg.Driver
		if driverName == "pgx" {
			driverName = "postgres"
		}
		conn, err := sql.Open("sqio_meta_test", driverName)
		return conn, cfg.Driver, err
	}
	for _, driver := range []string{"duckdb", "mysql", "pgx", "sqlserver", "oracle", "clickhouse"} {
		names, err := Schemas(context.Background(), Config{Driver: driver})
		if err != nil {
			t.Fatalf("%s schemas: %v", driver, err)
		}
		if len(names) == 0 {
			t.Fatalf("%s expected schema names", driver)
		}
	}
	if _, err := Schemas(context.Background(), Config{Driver: "unsupported"}); err == nil {
		t.Fatal("expected unsupported schemas driver error")
	}
}
