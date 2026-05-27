package service

import (
	"context"

	"github.com/isksss/sqio/internal/db"
)

// MigrationService applies and inspects SQL migrations.
type MigrationService struct {
	Driver string
	DSN    string
}

func (s MigrationService) Status(ctx context.Context, dir string) ([]db.Migration, error) {
	return db.MigrationStatus(ctx, db.Config{Driver: s.Driver, DSN: s.DSN}, dir)
}

func (s MigrationService) Apply(ctx context.Context, dir string, limit int) (db.MigrationResult, error) {
	return db.ApplyMigrations(ctx, db.Config{Driver: s.Driver, DSN: s.DSN}, dir, limit)
}

func (s MigrationService) Plan(ctx context.Context, dir string, rollbackLimit int) (db.MigrationPlan, error) {
	return db.PlanMigrations(ctx, db.Config{Driver: s.Driver, DSN: s.DSN}, dir, rollbackLimit)
}

func (s MigrationService) Rollback(ctx context.Context, dir string, limit int) (db.MigrationResult, error) {
	return db.RollbackMigrations(ctx, db.Config{Driver: s.Driver, DSN: s.DSN}, dir, limit)
}

func (s MigrationService) Baseline(ctx context.Context, dir, version string) (db.MigrationResult, error) {
	return db.BaselineMigrations(ctx, db.Config{Driver: s.Driver, DSN: s.DSN}, dir, version)
}
