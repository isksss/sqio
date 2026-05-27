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
	case strings.Contains(normalized, "information_schema.tables"):
		return &metaRows{cols: []string{"table_name"}, rows: [][]driver.Value{{"users"}}}
	case name == "mysql" && strings.Contains(normalized, "information_schema.columns"):
		return &metaRows{cols: []string{"column_name", "column_type", "is_nullable", "column_key", "column_default", "foreign_ref"}, rows: [][]driver.Value{
			{"id", "int", "NO", "PRI", nil, nil},
			{"email", "varchar(255)", "YES", "UNI", "''", nil},
			{"role_id", "int", "YES", "", nil, "roles(id)"},
		}}
	case name == "postgres" && strings.Contains(normalized, "pg_catalog.pg_attribute"):
		return &metaRows{cols: []string{"attname", "column_type", "is_nullable", "is_primary", "is_unique", "column_default", "foreign_ref"}, rows: [][]driver.Value{
			{"id", "integer", false, true, false, nil, nil},
			{"email", "character varying(255)", true, false, true, "''::character varying", nil},
			{"role_id", "integer", true, false, false, nil, "public.roles(id)"},
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
	schema, err := mysqlMetadata(context.Background(), conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Tables) != 1 || len(schema.Tables[0].Columns) != 3 {
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
	schema, err := postgresMetadata(context.Background(), conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Tables) != 1 || len(schema.Tables[0].Columns) != 3 {
		t.Fatalf("unexpected schema: %+v", schema)
	}
	if !strings.Contains(schema.Tables[0].DDL, `"email" character varying(255)`) {
		t.Fatalf("unexpected ddl: %s", schema.Tables[0].DDL)
	}
}
