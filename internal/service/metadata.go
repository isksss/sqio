package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/isksss/sqio/internal/db"
)

type MetadataService struct {
	schema Schema
	db     db.Config
}

type Schema struct {
	Tables []Table `json:"tables"`
}

type Table struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
	DDL     string   `json:"ddl"`
}

type Column struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Primary  bool   `json:"primary"`
}

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

func NewConnectedMetadataService(driver, dsn string) MetadataService {
	return MetadataService{db: db.Config{Driver: driver, DSN: dsn}}
}

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

func findColumns(schema Schema, tableName string) ([]Column, error) {
	table, ok := findTable(schema, tableName)
	if !ok {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}
	return table.Columns, nil
}

func findTable(schema Schema, tableName string) (Table, bool) {
	for _, table := range schema.Tables {
		if table.Name == tableName {
			return table, true
		}
	}
	return Table{}, false
}

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

func (s MetadataService) findTable(tableName string) (Table, bool) {
	for _, table := range s.schema.Tables {
		if table.Name == tableName {
			return table, true
		}
	}
	return Table{}, false
}

func fromDBSchema(schema db.SchemaInfo) Schema {
	tables := make([]Table, 0, len(schema.Tables))
	for _, table := range schema.Tables {
		columns := make([]Column, 0, len(table.Columns))
		for _, column := range table.Columns {
			columns = append(columns, Column{
				Name:     column.Name,
				Type:     column.Type,
				Nullable: column.Nullable,
				Primary:  column.Primary,
			})
		}
		tables = append(tables, Table{Name: table.Name, Columns: columns, DDL: table.DDL})
	}
	return Schema{Tables: tables}
}
