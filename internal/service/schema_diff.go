package service

import "fmt"

// SchemaDiff contains human-readable schema differences.
type SchemaDiff struct {
	Changes []SchemaChange `json:"changes"`
}

// SchemaChange describes one schema difference.
type SchemaChange struct {
	Type   string `json:"type"`
	Table  string `json:"table"`
	Name   string `json:"name,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// DiffSchemas compares two schema snapshots and reports table, column, and
// index additions/removals.
func DiffSchemas(from, to Schema) SchemaDiff {
	diff := SchemaDiff{}
	fromTables := map[string]Table{}
	toTables := map[string]Table{}
	for _, table := range from.Tables {
		fromTables[table.Name] = table
	}
	for _, table := range to.Tables {
		toTables[table.Name] = table
	}
	for name := range fromTables {
		if _, ok := toTables[name]; !ok {
			diff.Changes = append(diff.Changes, SchemaChange{Type: "drop_table", Table: name})
		}
	}
	for name, table := range toTables {
		fromTable, ok := fromTables[name]
		if !ok {
			diff.Changes = append(diff.Changes, SchemaChange{Type: "add_table", Table: name})
			continue
		}
		diff.Changes = append(diff.Changes, diffColumns(fromTable, table)...)
		diff.Changes = append(diff.Changes, diffIndexes(fromTable, table)...)
	}
	return diff
}

func diffColumns(from, to Table) []SchemaChange {
	changes := []SchemaChange{}
	fromColumns := map[string]Column{}
	toColumns := map[string]Column{}
	for _, column := range from.Columns {
		fromColumns[column.Name] = column
	}
	for _, column := range to.Columns {
		toColumns[column.Name] = column
	}
	for name := range fromColumns {
		if _, ok := toColumns[name]; !ok {
			changes = append(changes, SchemaChange{Type: "drop_column", Table: from.Name, Name: name})
		}
	}
	for name, column := range toColumns {
		fromColumn, ok := fromColumns[name]
		if !ok {
			changes = append(changes, SchemaChange{Type: "add_column", Table: to.Name, Name: name, Detail: column.Type})
			continue
		}
		if column.Type != fromColumn.Type || column.Nullable != fromColumn.Nullable || column.Default != fromColumn.Default {
			changes = append(changes, SchemaChange{Type: "change_column", Table: to.Name, Name: name, Detail: fmt.Sprintf("%s -> %s", fromColumn.Type, column.Type)})
		}
	}
	return changes
}

func diffIndexes(from, to Table) []SchemaChange {
	changes := []SchemaChange{}
	fromIndexes := map[string]Index{}
	toIndexes := map[string]Index{}
	for _, index := range from.Indexes {
		fromIndexes[index.Name] = index
	}
	for _, index := range to.Indexes {
		toIndexes[index.Name] = index
	}
	for name := range fromIndexes {
		if _, ok := toIndexes[name]; !ok {
			changes = append(changes, SchemaChange{Type: "drop_index", Table: from.Name, Name: name})
		}
	}
	for name, index := range toIndexes {
		if _, ok := fromIndexes[name]; !ok {
			changes = append(changes, SchemaChange{Type: "add_index", Table: to.Name, Name: name, Detail: fmt.Sprint(index.Columns)})
		}
	}
	return changes
}
