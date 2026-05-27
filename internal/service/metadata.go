package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/isksss/sqio/internal/db"
)

// MetadataService serves schema metadata from either a live database or an
// in-memory fallback schema.
type MetadataService struct {
	schema Schema
	db     db.Config
}

// Schema is the service-layer representation of database metadata.
type Schema struct {
	Tables []Table `json:"tables"`
}

// Table describes one table or view exposed to CLI and TUI callers.
type Table struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
	DDL     string   `json:"ddl"`
}

// Column describes one table column and common relational constraints.
type Column struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Nullable   bool   `json:"nullable"`
	Primary    bool   `json:"primary"`
	Unique     bool   `json:"unique,omitempty"`
	Default    string `json:"default,omitempty"`
	References string `json:"references,omitempty"`
}

// NewMetadataService returns an in-memory metadata service used when no live
// database connection is configured.
func NewMetadataService() MetadataService {
	return MetadataService{
		schema: Schema{
			Tables: []Table{
				{
					Name: "users",
					Columns: []Column{
						{Name: "id", Type: "integer", Primary: true},
						{Name: "name", Type: "text"},
						{Name: "email", Type: "text", Nullable: true},
					},
					DDL: "CREATE TABLE users (id integer primary key, name text not null, email text);",
				},
				{
					Name: "posts",
					Columns: []Column{
						{Name: "id", Type: "integer", Primary: true},
						{Name: "user_id", Type: "integer"},
						{Name: "title", Type: "text"},
					},
					DDL: "CREATE TABLE posts (id integer primary key, user_id integer not null, title text not null);",
				},
			},
		},
	}
}

// NewConnectedMetadataService returns a metadata service backed by a live
// database connection.
func NewConnectedMetadataService(driver, dsn string) MetadataService {
	return MetadataService{db: db.Config{Driver: driver, DSN: dsn}}
}

// Tables returns all known tables for the configured metadata source.
func (s MetadataService) Tables(ctx context.Context) ([]Table, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.db.Driver != "" || s.db.DSN != "" {
		schema, err := s.Schema(ctx)
		if err != nil {
			return nil, err
		}
		return schema.Tables, nil
	}
	return s.schema.Tables, nil
}

// Columns returns metadata for columns in tableName.
func (s MetadataService) Columns(ctx context.Context, tableName string) ([]Column, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.db.Driver != "" || s.db.DSN != "" {
		schema, err := s.Schema(ctx)
		if err != nil {
			return nil, err
		}
		return findColumns(schema, tableName)
	}
	table, ok := s.findTable(tableName)
	if !ok {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}
	return table.Columns, nil
}

// DDL returns a CREATE TABLE statement for tableName when metadata is available.
func (s MetadataService) DDL(ctx context.Context, tableName string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s.db.Driver != "" || s.db.DSN != "" {
		schema, err := s.Schema(ctx)
		if err != nil {
			return "", err
		}
		table, ok := findTable(schema, tableName)
		if !ok {
			return "", fmt.Errorf("table not found: %s", tableName)
		}
		return table.DDL, nil
	}
	table, ok := s.findTable(tableName)
	if !ok {
		return "", fmt.Errorf("table not found: %s", tableName)
	}
	return table.DDL, nil
}

// findColumns returns columns for tableName from schema.
func findColumns(schema Schema, tableName string) ([]Column, error) {
	table, ok := findTable(schema, tableName)
	if !ok {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}
	return table.Columns, nil
}

// findTable searches schema for tableName.
func findTable(schema Schema, tableName string) (Table, bool) {
	for _, table := range schema.Tables {
		if table.Name == tableName {
			return table, true
		}
	}
	return Table{}, false
}

// Schema returns the complete schema for the configured metadata source.
func (s MetadataService) Schema(ctx context.Context) (Schema, error) {
	if err := ctx.Err(); err != nil {
		return Schema{}, err
	}
	if s.db.Driver != "" || s.db.DSN != "" {
		schema, err := db.Metadata(ctx, s.db)
		if err != nil {
			return Schema{}, err
		}
		return fromDBSchema(schema), nil
	}
	return s.schema, nil
}

// MermaidER renders the schema as a Mermaid ER diagram.
func (s MetadataService) MermaidER(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	schema, err := s.Schema(ctx)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("erDiagram\n")
	for _, table := range schema.Tables {
		fmt.Fprintf(&b, "  %s {\n", table.Name)
		for _, column := range table.Columns {
			suffix := ""
			if column.Primary {
				suffix = " PK"
			}
			fmt.Fprintf(&b, "    %s %s%s\n", column.Type, column.Name, suffix)
		}
		b.WriteString("  }\n")
	}
	return b.String(), nil
}

// findTable searches the in-memory fallback schema for tableName.
func (s MetadataService) findTable(tableName string) (Table, bool) {
	for _, table := range s.schema.Tables {
		if table.Name == tableName {
			return table, true
		}
	}
	return Table{}, false
}

// fromDBSchema converts database-layer metadata into service-layer metadata.
func fromDBSchema(schema db.SchemaInfo) Schema {
	tables := make([]Table, 0, len(schema.Tables))
	for _, table := range schema.Tables {
		columns := make([]Column, 0, len(table.Columns))
		for _, column := range table.Columns {
			columns = append(columns, Column{
				Name:       column.Name,
				Type:       column.Type,
				Nullable:   column.Nullable,
				Primary:    column.Primary,
				Unique:     column.Unique,
				Default:    column.Default,
				References: column.References,
			})
		}
		tables = append(tables, Table{Name: table.Name, Columns: columns, DDL: table.DDL})
	}
	return Schema{Tables: tables}
}
