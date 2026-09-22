package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/portico/backend/internal/bootstrap"
	"github.com/portico/backend/internal/models"
	"github.com/portico/backend/internal/services/connection"
	"gorm.io/datatypes"
)

func TestImportConnectionsIdempotent(t *testing.T) {
	gdb := setupTestDB(t)
	svc := connection.NewService(gdb)

	dir := t.TempDir()
	path := filepath.Join(dir, "connections.json")
	writeConnectionsFile(t, path, []map[string]any{
		{
			"name": "local-postgres",
			"type": "postgres",
			"config": map[string]any{
				"host":     "localhost",
				"port":     5432,
				"user":     "postgres",
				"password": "secret1",
				"database": "example",
				"sslmode":  "disable",
			},
		},
	})

	first, err := bootstrap.ImportConnections(svc, path)
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	if len(first) != 1 || first[0].Action != "created" {
		t.Fatalf("first import: got %+v", first)
	}

	writeConnectionsFile(t, path, []map[string]any{
		{
			"name": "local-postgres",
			"type": "postgres",
			"config": map[string]any{
				"host":     "db.internal",
				"port":     5432,
				"user":     "postgres",
				"password": "secret2",
				"database": "example",
				"sslmode":  "disable",
			},
		},
	})

	second, err := bootstrap.ImportConnections(svc, path)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if len(second) != 1 || second[0].Action != "updated" || second[0].ID != first[0].ID {
		t.Fatalf("second import: got %+v want updated id=%d", second, first[0].ID)
	}

	var count int64
	if err := gdb.Model(&models.Connection{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 connection, got %d", count)
	}

	got, err := svc.Get(first[0].ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(got.Config, &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfg["host"] != "db.internal" {
		t.Fatalf("host=%v want db.internal", cfg["host"])
	}
	if cfg["password"] != "secret2" {
		t.Fatalf("password=%v want secret2", cfg["password"])
	}
}

func TestImportConnectionsAmbiguous(t *testing.T) {
	gdb := setupTestDB(t)
	svc := connection.NewService(gdb)

	cfg, _ := json.Marshal(map[string]any{"host": "localhost", "password": "x"})
	for i := 0; i < 2; i++ {
		if err := gdb.Create(&models.Connection{
			Name:   "dup",
			Type:   "postgres",
			Config: datatypes.JSON(cfg),
		}).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	path := filepath.Join(t.TempDir(), "connections.json")
	writeConnectionsFile(t, path, []map[string]any{
		{
			"name":   "dup",
			"type":   "postgres",
			"config": map[string]any{"host": "localhost", "password": "y"},
		},
	})

	if _, err := bootstrap.ImportConnections(svc, path); err == nil {
		t.Fatal("expected ambiguous error")
	}
}

func writeConnectionsFile(t *testing.T, path string, connections []map[string]any) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"connections": connections})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}
