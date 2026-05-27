package service

import (
	"context"

	"github.com/isksss/sqio/internal/db"
)

// DataEditService performs row-level table edits.
type DataEditService struct {
	Driver string
	DSN    string
}

func (s DataEditService) Insert(ctx context.Context, tableName string, values map[string]string) (int, error) {
	return db.InsertRow(ctx, db.Config{Driver: s.Driver, DSN: s.DSN}, tableName, values)
}

func (s DataEditService) Update(ctx context.Context, tableName string, values map[string]string, whereClause string) (int, error) {
	return db.UpdateRows(ctx, db.Config{Driver: s.Driver, DSN: s.DSN}, tableName, values, whereClause)
}

func (s DataEditService) Delete(ctx context.Context, tableName, whereClause string) (int, error) {
	return db.DeleteRows(ctx, db.Config{Driver: s.Driver, DSN: s.DSN}, tableName, whereClause)
}
