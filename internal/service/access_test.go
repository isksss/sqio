package service

import (
	"context"
	"path/filepath"
	"testing"
)

func TestAccessServiceSQLite(t *testing.T) {
	service := AccessService{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "test.db")}
	roles, err := service.Roles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 0 {
		t.Fatalf("expected empty sqlite roles, got %+v", roles)
	}
	grants, err := service.Grants(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 0 {
		t.Fatalf("expected empty sqlite grants, got %+v", grants)
	}
}
