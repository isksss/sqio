package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/isksss/sqio/internal/db"
)

func TestDataEditService(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	if _, err := db.Execute(context.Background(), db.Config{Driver: "sqlite", DSN: path}, "create table users (id integer primary key, name text)", db.ExecuteOptions{}); err != nil {
		t.Fatal(err)
	}
	service := DataEditService{Driver: "sqlite", DSN: path}
	if affected, err := service.Insert(context.Background(), "users", map[string]string{"name": "alice"}); err != nil || affected != 1 {
		t.Fatalf("unexpected insert affected=%d err=%v", affected, err)
	}
	if affected, err := service.Update(context.Background(), "users", map[string]string{"name": "bob"}, "name = 'alice'"); err != nil || affected != 1 {
		t.Fatalf("unexpected update affected=%d err=%v", affected, err)
	}
	if affected, err := service.Delete(context.Background(), "users", "name = 'bob'"); err != nil || affected != 1 {
		t.Fatalf("unexpected delete affected=%d err=%v", affected, err)
	}
}
