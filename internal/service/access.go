package service

import (
	"context"

	"github.com/isksss/sqio/internal/db"
)

// AccessService reads database role and grant metadata.
type AccessService struct {
	Driver string
	DSN    string
}

func (s AccessService) Roles(ctx context.Context) ([]db.RoleInfo, error) {
	return db.Roles(ctx, db.Config{Driver: s.Driver, DSN: s.DSN})
}

func (s AccessService) Grants(ctx context.Context, role string) ([]db.GrantInfo, error) {
	return db.Grants(ctx, db.Config{Driver: s.Driver, DSN: s.DSN}, role)
}
