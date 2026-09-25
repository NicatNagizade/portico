package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/portico/backend/internal/bootstrap"
	"github.com/portico/backend/internal/models"
	"github.com/portico/backend/internal/services/connection"
	"github.com/portico/backend/internal/services/syncjob"
	"gorm.io/datatypes"
)

func TestImportConnectionsIdempotent(t *testing.T) {
	gdb := setupTestDB(t)
	connSvc := connection.NewService(gdb)
	jobSvc := syncjob.NewService(gdb)

	dir := t.TempDir()
	path := filepath.Join(dir, "connections.json")
	writeImportFile(t, path, map[string]any{
		"connections": []map[string]any{
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
		},
	})

	first, err := bootstrap.ImportConnections(connSvc, jobSvc, path)
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	if len(first.Connections) != 1 || first.Connections[0].Action != "created" {
		t.Fatalf("first import: got %+v", first)
	}

	writeImportFile(t, path, map[string]any{
		"connections": []map[string]any{
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
		},
	})

	second, err := bootstrap.ImportConnections(connSvc, jobSvc, path)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if len(second.Connections) != 1 || second.Connections[0].Action != "updated" || second.Connections[0].ID != first.Connections[0].ID {
		t.Fatalf("second import: got %+v want updated id=%d", second, first.Connections[0].ID)
	}

	var count int64
	if err := gdb.Model(&models.Connection{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 connection, got %d", count)
	}

	got, err := connSvc.Get(first.Connections[0].ID)
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
	connSvc := connection.NewService(gdb)
	jobSvc := syncjob.NewService(gdb)

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
	writeImportFile(t, path, map[string]any{
		"connections": []map[string]any{
			{
				"name":   "dup",
				"type":   "postgres",
				"config": map[string]any{"host": "localhost", "password": "y"},
			},
		},
	})

	if _, err := bootstrap.ImportConnections(connSvc, jobSvc, path); err == nil {
		t.Fatal("expected ambiguous error")
	}
}

func TestImportSyncJobsWithNested(t *testing.T) {
	gdb := setupTestDB(t)
	connSvc := connection.NewService(gdb)
	jobSvc := syncjob.NewService(gdb)

	path := filepath.Join(t.TempDir(), "connections.json")
	writeImportFile(t, path, map[string]any{
		"connections": []map[string]any{
			{
				"name": "src",
				"type": "postgres",
				"config": map[string]any{
					"host": "localhost", "port": 5432, "user": "u", "password": "p",
					"database": "db", "sslmode": "disable",
				},
			},
			{
				"name": "dst",
				"type": "typesense",
				"config": map[string]any{
					"host": "localhost", "port": 8108, "protocol": "http", "api_key": "xyz",
				},
			},
		},
		"sync_jobs": []map[string]any{
			{
				"name":                    "users-job",
				"source_connection":       "src",
				"source_table":            "users",
				"destination_connection":  "dst",
				"destination_table":       "users",
				"chunk_size":              100,
				"workers":                 1,
				"relations": []map[string]any{
					{
						"name": "posts", "type": "has_many",
						"table": "posts", "foreign_key": "user_id", "related_key": "id",
						"fields": []map[string]any{
							{
								"source_name":      "title",
								"destination_name": "title",
								"destination_type": "string",
							},
						},
						"relations": []map[string]any{
							{
								"name": "comments", "type": "has_many", "table": "comments",
							},
						},
					},
				},
				"fields": []map[string]any{
					{
						"source_name": "status", "destination_name": "status", "destination_type": "string",
						"values": []map[string]any{
							{"source_value": "1", "destination_value": "success"},
							{"source_value": "2", "destination_value": "failed"},
						},
					},
				},
				"rules": []map[string]any{
					{"field": "id", "operator": "gt", "value": "0"},
				},
			},
		},
	})

	first, err := bootstrap.ImportConnections(connSvc, jobSvc, path)
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	if len(first.SyncJobs) != 1 || first.SyncJobs[0].Action != "created" {
		t.Fatalf("first sync jobs: %+v", first.SyncJobs)
	}

	job, err := jobSvc.Get(first.SyncJobs[0].ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if job.ChunkSize != 100 || job.Workers != 1 {
		t.Fatalf("chunk/workers = %d/%d", job.ChunkSize, job.Workers)
	}
	if len(job.Relations) != 1 || job.Relations[0].Name != "posts" {
		t.Fatalf("relations: %+v", job.Relations)
	}
	if len(job.Relations[0].Relations) != 1 || job.Relations[0].Relations[0].Name != "comments" {
		t.Fatalf("nested relations: %+v", job.Relations[0].Relations)
	}
	if len(job.Fields) != 1 || job.Fields[0].SourceName != "status" {
		t.Fatalf("root fields: %+v", job.Fields)
	}
	if len(job.Fields[0].Values) != 2 {
		t.Fatalf("field values: %+v", job.Fields[0].Values)
	}
	if len(job.Relations[0].Fields) != 1 || job.Relations[0].Fields[0].SourceName != "title" {
		t.Fatalf("relation fields: %+v", job.Relations[0].Fields)
	}
	if len(job.Rules) != 1 || job.Rules[0].Operator != "gt" {
		t.Fatalf("rules: %+v", job.Rules)
	}

	// Idempotent update: tweak rule value and field mapping.
	writeImportFile(t, path, map[string]any{
		"connections": []map[string]any{
			{
				"name": "src",
				"type": "postgres",
				"config": map[string]any{
					"host": "localhost", "port": 5432, "user": "u", "password": "p",
					"database": "db", "sslmode": "disable",
				},
			},
			{
				"name": "dst",
				"type": "typesense",
				"config": map[string]any{
					"host": "localhost", "port": 8108, "protocol": "http", "api_key": "xyz",
				},
			},
		},
		"sync_jobs": []map[string]any{
			{
				"name":                   "users-job",
				"source_connection":      "src",
				"source_table":           "users",
				"destination_connection": "dst",
				"destination_table":      "users_v2",
				"fields": []map[string]any{
					{"source_name": "username", "destination_name": "username", "destination_type": "string"},
				},
				"rules": []map[string]any{
					{"field": "id", "operator": "gte", "value": "10"},
				},
			},
		},
	})

	second, err := bootstrap.ImportConnections(connSvc, jobSvc, path)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if len(second.SyncJobs) != 1 || second.SyncJobs[0].Action != "updated" || second.SyncJobs[0].ID != first.SyncJobs[0].ID {
		t.Fatalf("second sync jobs: %+v", second.SyncJobs)
	}

	job, err = jobSvc.Get(first.SyncJobs[0].ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if job.DestinationTable != "users_v2" {
		t.Fatalf("destination_table=%q", job.DestinationTable)
	}
	if len(job.Relations) != 0 {
		t.Fatalf("expected relations cleared, got %+v", job.Relations)
	}
	if len(job.Fields) != 1 || job.Fields[0].SourceName != "username" {
		t.Fatalf("fields after update: %+v", job.Fields)
	}
	if len(job.Rules) != 1 || job.Rules[0].Operator != "gte" || job.Rules[0].Value != "10" {
		t.Fatalf("rules after update: %+v", job.Rules)
	}

	var jobCount int64
	if err := gdb.Model(&models.SyncJob{}).Count(&jobCount).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if jobCount != 1 {
		t.Fatalf("expected 1 sync job, got %d", jobCount)
	}
}

func writeImportFile(t *testing.T, path string, body map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}
