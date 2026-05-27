package service

import (
	"context"
	"sort"
	"strings"
)

// Completion describes one SQL completion candidate.
type Completion struct {
	Value string `json:"value"`
	Kind  string `json:"kind"`
	Table string `json:"table,omitempty"`
}

var sqlKeywords = []string{
	"SELECT", "FROM", "WHERE", "JOIN", "LEFT JOIN", "INNER JOIN", "GROUP BY",
	"ORDER BY", "LIMIT", "INSERT", "UPDATE", "DELETE", "CREATE TABLE",
	"ALTER TABLE", "DROP TABLE", "EXPLAIN",
}

// Complete returns keyword, table, and column candidates matching prefix.
func (s MetadataService) Complete(ctx context.Context, prefix, tableName string) ([]Completion, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	token := completionToken(prefix)
	tokenLower := strings.ToLower(token)
	candidates := []Completion{}
	for _, keyword := range sqlKeywords {
		if completionMatches(keyword, tokenLower) {
			candidates = append(candidates, Completion{Value: keyword, Kind: "keyword"})
		}
	}
	schema, err := s.Schema(ctx)
	if err != nil {
		return nil, err
	}
	for _, table := range schema.Tables {
		if tableName == "" && completionMatches(table.Name, tokenLower) {
			candidates = append(candidates, Completion{Value: table.Name, Kind: "table"})
		}
		if tableName != "" && table.Name != tableName {
			continue
		}
		for _, column := range table.Columns {
			if completionMatches(column.Name, tokenLower) {
				candidates = append(candidates, Completion{Value: column.Name, Kind: "column", Table: table.Name})
			}
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Kind != candidates[j].Kind {
			return candidates[i].Kind < candidates[j].Kind
		}
		if candidates[i].Table != candidates[j].Table {
			return candidates[i].Table < candidates[j].Table
		}
		return candidates[i].Value < candidates[j].Value
	})
	return candidates, nil
}

func completionToken(prefix string) string {
	fields := strings.FieldsFunc(prefix, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ',' || r == '(' || r == ')'
	})
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

func completionMatches(value, tokenLower string) bool {
	if tokenLower == "" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(value), tokenLower)
}
