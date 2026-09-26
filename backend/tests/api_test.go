package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/db"
	"github.com/portico/backend/internal/handlers"
	"github.com/portico/backend/internal/models"
	"github.com/portico/backend/internal/router"
	"github.com/portico/backend/internal/secretbox"
	"github.com/portico/backend/internal/services/connection"
	syncsvc "github.com/portico/backend/internal/services/sync"
	"github.com/portico/backend/internal/services/syncjob"
	"github.com/portico/backend/internal/services/synclog"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	gdb, err := db.Connect("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()))
	if err != nil {
		t.Fatalf("connect sqlite: %v", err)
	}
	return gdb
}

func setupRouter(t *testing.T, gdb *gorm.DB, registry *connectors.Registry) *gin.Engine {
	t.Helper()
	h := handlers.New(
		connection.NewService(gdb),
		syncjob.NewService(gdb),
		synclog.NewService(gdb),
		syncsvc.NewOrchestrator(gdb, registry),
		registry,
	)
	return router.New(h)
}

func pivotTable(cfg datatypes.JSON) string {
	var m map[string]any
	if err := json.Unmarshal(cfg, &m); err != nil {
		return ""
	}
	v, _ := m["pivot_table"].(string)
	return v
}

func TestHealth(t *testing.T) {
	gdb := setupTestDB(t)
	r := setupRouter(t, gdb, connectors.NewRegistry())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestConnectionsCRUD(t *testing.T) {
	gdb := setupTestDB(t)
	r := setupRouter(t, gdb, connectors.NewRegistry())

	body := map[string]any{
		"name": "mysql-src",
		"type": "mysql",
		"config": map[string]any{
			"host":     "localhost",
			"port":     3306,
			"user":     "root",
			"password": "secret",
			"database": "app",
		},
	}
	payload, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/connections", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d body=%s", w.Code, w.Body.String())
	}

	var created models.Connection
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.ID == 0 || created.Name != "mysql-src" {
		t.Fatalf("unexpected connection: %+v", created)
	}
	if bytes.Contains([]byte(fmt.Sprint(created)), []byte("secret")) || bytes.Contains(w.Body.Bytes(), []byte("secret")) {
		t.Fatalf("create leaked password: %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/connections", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/connections/%d", created.ID), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", w.Code)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("secret")) {
		t.Fatalf("get leaked password: %s", w.Body.String())
	}

	var stored models.Connection
	if err := gdb.Session(&gorm.Session{SkipHooks: true}).First(&stored, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stored.Config, []byte("secret")) {
		t.Fatalf("password stored in plaintext: %s", stored.Config)
	}
	var loaded models.Connection
	if err := gdb.First(&loaded, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(loaded.Config, []byte(`"password":"secret"`)) {
		t.Fatalf("loaded config should decrypt the password: %s", loaded.Config)
	}

	update := map[string]any{
		"name": "mysql-renamed",
		"config": map[string]any{
			"host":     "db.internal",
			"port":     3306,
			"user":     "root",
			"password": "",
			"database": "app",
		},
	}
	up, _ := json.Marshal(update)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/connections/%d", created.ID), bytes.NewReader(up))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if bytes.Contains(w.Body.Bytes(), []byte("secret")) {
		t.Fatalf("update leaked password: %s", w.Body.String())
	}
	if err := gdb.Session(&gorm.Session{SkipHooks: true}).First(&stored, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	opened, err := secretbox.Open(stored.Config)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(opened, []byte(`"password":"secret"`)) || !bytes.Contains(opened, []byte(`"host":"db.internal"`)) {
		t.Fatalf("blank password update should keep the secret: %s", opened)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/connections/%d", created.ID), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", w.Code)
	}
}

func TestCheckConnection(t *testing.T) {
	gdb := setupTestDB(t)

	registry := connectors.NewRegistry()
	registry.RegisterSource("mysql", func(conn *models.Connection) (connectors.SourceReader, error) {
		return &mockSource{}, nil
	})
	r := setupRouter(t, gdb, registry)

	body := map[string]any{
		"type": "mysql",
		"config": map[string]any{
			"host":     "localhost",
			"port":     3306,
			"user":     "root",
			"password": "secret",
			"database": "app",
		},
	}
	payload, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/connections/check", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("check: expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	createPayload, _ := json.Marshal(map[string]any{
		"name":   "mysql-src",
		"type":   "mysql",
		"config": body["config"],
	})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/connections", bytes.NewReader(createPayload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	var created models.Connection
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}

	editCheck, _ := json.Marshal(map[string]any{
		"id":   created.ID,
		"type": "mysql",
		"config": map[string]any{
			"host":     "localhost",
			"port":     3306,
			"user":     "root",
			"password": "",
			"database": "app",
		},
	})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/connections/check", bytes.NewReader(editCheck))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("check with blank secret: expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	failRegistry := connectors.NewRegistry()
	failRegistry.RegisterSource("mysql", func(conn *models.Connection) (connectors.SourceReader, error) {
		return &failingSource{err: errors.New("dial refused")}, nil
	})
	failRouter := setupRouter(t, gdb, failRegistry)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/connections/check", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	failRouter.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("failed check: expected 500, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestConnectionTablesAndColumns(t *testing.T) {
	gdb := setupTestDB(t)

	src := &mockSource{
		schemas: map[string]*connectors.TableSchema{
			"users": {
				Columns: []connectors.ColumnSchema{
					{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
					{Name: "email", Type: connectors.FieldTypeString},
				},
			},
			"posts": {
				Columns: []connectors.ColumnSchema{
					{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
					{Name: "title", Type: connectors.FieldTypeString},
				},
			},
		},
	}
	registry := connectors.NewRegistry()
	registry.RegisterSource("mock_src", func(conn *models.Connection) (connectors.SourceReader, error) {
		return src, nil
	})
	r := setupRouter(t, gdb, registry)

	conn := models.Connection{
		Name:   "src",
		Type:   "mock_src",
		Config: datatypes.JSON([]byte(`{}`)),
	}
	if err := gdb.Create(&conn).Error; err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/connections/%d/tables", conn.ID), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("tables: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var tablesResp struct {
		Tables []string `json:"tables"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &tablesResp); err != nil {
		t.Fatalf("decode tables: %v", err)
	}
	if len(tablesResp.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %v", tablesResp.Tables)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/connections/%d/columns?table=users", conn.ID), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("columns: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var columnsResp struct {
		Columns []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"columns"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &columnsResp); err != nil {
		t.Fatalf("decode columns: %v", err)
	}
	if len(columnsResp.Columns) != 2 ||
		columnsResp.Columns[0].Name != "id" || columnsResp.Columns[0].Type != "int64" ||
		columnsResp.Columns[1].Name != "email" || columnsResp.Columns[1].Type != "string" {
		t.Fatalf("unexpected columns: %+v", columnsResp.Columns)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/connections/%d/columns", conn.ID), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing table: expected 400, got %d", w.Code)
	}
}

func TestSyncJobsAndLogs(t *testing.T) {
	gdb := setupTestDB(t)
	r := setupRouter(t, gdb, connectors.NewRegistry())

	src := models.Connection{
		Name:   "src",
		Type:   models.ConnectionTypeMySQL,
		Config: datatypes.JSON([]byte(`{"host":"localhost","port":3306,"user":"u","password":"p","database":"db"}`)),
	}
	dst := models.Connection{
		Name:   "dst",
		Type:   models.ConnectionTypeTypesense,
		Config: datatypes.JSON([]byte(`{"host":"localhost","port":8108,"api_key":"xyz"}`)),
	}
	if err := gdb.Create(&src).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&dst).Error; err != nil {
		t.Fatal(err)
	}

	body := map[string]any{
		"name":                      "applicants-to-typesense",
		"source_connection_id":      src.ID,
		"source_table":              "applicants",
		"destination_connection_id": dst.ID,
		"destination_table":         "applicants",
		"chunk_size":                100,
		"workers":                   2,
		"config": map[string]any{
			"enable_nested_fields": true,
		},
		"relations": []map[string]any{
			{
				"name":   "tags",
				"type":   "belongs_to_many",
				"table":  "tags",
				"config": map[string]any{"pivot_table": "applicant_tags"},
			},
			{
				"name":   "skills",
				"type":   "belongs_to_many",
				"table":  "skills",
				"config": map[string]any{"pivot_table": "applicant_skills"},
				"active": false,
			},
		},
		"fields": []map[string]any{
			{
				"source_name":      "full_name",
				"destination_name": "name",
				"destination_type": "string",
			},
			{
				"source_name":      "score",
				"destination_type": "float64",
				"active":           true,
			},
			{
				"source_name":      "internal_notes",
				"destination_name": "notes",
				"active":           false,
			},
			{
				"source_name":      "status",
				"destination_type": "string",
				"values": []map[string]any{
					{"source_value": "1", "destination_value": "success"},
					{"source_value": "2", "destination_value": "failed"},
				},
			},
		},
		"rules": []map[string]any{
			{
				"field":    "client_id",
				"operator": "eq",
				"value":    "123",
			},
			{
				"field":    "status",
				"operator": "neq",
				"value":    "archived",
				"active":   false,
			},
		},
	}
	payload, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sync-jobs", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create job: expected 201, got %d body=%s", w.Code, w.Body.String())
	}

	var job models.SyncJob
	if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if len(job.Relations) != 2 {
		t.Fatalf("expected 2 relations, got %+v", job.Relations)
	}
	if job.Relations[0].Name != "tags" || pivotTable(job.Relations[0].Config) != "applicant_tags" || !job.Relations[0].IsActive() {
		t.Fatalf("expected active tags relation, got %+v", job.Relations[0])
	}
	if job.Relations[1].Name != "skills" || job.Relations[1].IsActive() {
		t.Fatalf("expected inactive skills relation, got %+v", job.Relations[1])
	}
	if len(job.Fields) != 4 {
		t.Fatalf("expected 4 fields, got %+v", job.Fields)
	}
	if job.Fields[0].SourceName != "full_name" || job.Fields[0].DestinationName != "name" || !job.Fields[0].IsActive() {
		t.Fatalf("unexpected first field: %+v", job.Fields[0])
	}
	if job.Fields[1].SourceName != "score" || job.Fields[1].DestinationType != "float64" {
		t.Fatalf("unexpected second field: %+v", job.Fields[1])
	}
	if job.Fields[2].SourceName != "internal_notes" || job.Fields[2].IsActive() {
		t.Fatalf("expected active=false field preserved, got %+v", job.Fields[2])
	}
	if job.Fields[3].SourceName != "status" || len(job.Fields[3].Values) != 2 {
		t.Fatalf("expected status field with 2 value maps, got %+v", job.Fields[3])
	}
	if job.Fields[3].Values[0].SourceValue != "1" || job.Fields[3].Values[0].DestinationValue != "success" {
		t.Fatalf("unexpected first value map: %+v", job.Fields[3].Values[0])
	}
	if job.Fields[3].Values[1].SourceValue != "2" || job.Fields[3].Values[1].DestinationValue != "failed" {
		t.Fatalf("unexpected second value map: %+v", job.Fields[3].Values[1])
	}
	if len(job.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %+v", job.Rules)
	}
	if job.Rules[0].Field != "client_id" || job.Rules[0].Operator != "eq" || job.Rules[0].Value != "123" || !job.Rules[0].IsActive() {
		t.Fatalf("unexpected first rule: %+v", job.Rules[0])
	}
	if job.Rules[1].Field != "status" || job.Rules[1].IsActive() {
		t.Fatalf("expected inactive status rule, got %+v", job.Rules[1])
	}
	var jobCfg map[string]any
	if err := json.Unmarshal(job.Config, &jobCfg); err != nil {
		t.Fatalf("decode job config: %v", err)
	}
	if jobCfg["enable_nested_fields"] != true {
		t.Fatalf("expected enable_nested_fields=true, got %+v", jobCfg)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sync-jobs/%d", job.ID), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get job: %d", w.Code)
	}
	if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if len(job.Relations) != 2 {
		t.Fatalf("expected preloaded relations, got %+v", job.Relations)
	}
	if len(job.Fields) != 4 {
		t.Fatalf("expected preloaded fields, got %+v", job.Fields)
	}
	if len(job.Fields[3].Values) != 2 {
		t.Fatalf("expected preloaded field values, got %+v", job.Fields[3].Values)
	}
	if len(job.Rules) != 2 {
		t.Fatalf("expected preloaded rules, got %+v", job.Rules)
	}

	update := map[string]any{
		"relations": []map[string]any{
			{
				"name":        "tags",
				"type":        "belongs_to_many",
				"table":       "tags",
				"config":      map[string]any{"pivot_table": "applicant_tags"},
				"foreign_key": "applicant_id",
				"related_key": "tag_id",
			},
		},
		"fields": []map[string]any{
			{
				"source_name":      "full_name",
				"destination_name": "display_name",
				"destination_type": "string",
			},
		},
		"rules": []map[string]any{
			{
				"field":    "client_id",
				"operator": "eq",
				"value":    "456",
			},
		},
	}
	up, _ := json.Marshal(update)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/sync-jobs/%d", job.ID), bytes.NewReader(up))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update job: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if len(job.Relations) != 1 || job.Relations[0].ForeignKey != "applicant_id" {
		t.Fatalf("expected updated foreign_key, got %+v", job.Relations)
	}
	if len(job.Fields) != 1 || job.Fields[0].DestinationName != "display_name" {
		t.Fatalf("expected replaced fields, got %+v", job.Fields)
	}
	if len(job.Rules) != 1 || job.Rules[0].Value != "456" {
		t.Fatalf("expected replaced rules, got %+v", job.Rules)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/sync-jobs?page=1&page_size=10", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list jobs: %d", w.Code)
	}
	var jobPage struct {
		Items      []models.SyncJob `json:"items"`
		Page       int              `json:"page"`
		PageSize   int              `json:"page_size"`
		Total      int64            `json:"total"`
		TotalPages int              `json:"total_pages"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &jobPage); err != nil {
		t.Fatal(err)
	}
	if jobPage.Total < 1 || len(jobPage.Items) < 1 || jobPage.Page != 1 {
		t.Fatalf("unexpected job page: %+v", jobPage)
	}

	// create a log row directly and fetch via API
	now := time.Now()
	rows := int64(10)
	dur := int64(42)
	logEntry := models.SyncLog{
		SyncJobID:  job.ID,
		Status:     models.SyncLogStatusSuccess,
		Message:    "ok",
		RowsTotal:  &rows,
		RowsSynced: &rows,
		DurationMs: &dur,
		StartedAt:  now,
		FinishedAt: &now,
	}
	if err := gdb.Create(&logEntry).Error; err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sync-logs?sync_job_id=%d&page=1&page_size=10", job.ID), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list logs: %d", w.Code)
	}
	var logPage struct {
		Items      []models.SyncLog `json:"items"`
		Page       int              `json:"page"`
		PageSize   int              `json:"page_size"`
		Total      int64            `json:"total"`
		TotalPages int              `json:"total_pages"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &logPage); err != nil {
		t.Fatal(err)
	}
	if logPage.Total != 1 || len(logPage.Items) != 1 || logPage.Items[0].DurationMs == nil || *logPage.Items[0].DurationMs != 42 {
		t.Fatalf("unexpected logs: %+v", logPage)
	}
	if logPage.Items[0].SyncJob == nil || logPage.Items[0].SyncJob.Name == "" {
		t.Fatalf("expected sync job preload on log list, got %+v", logPage.Items[0].SyncJob)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sync-logs/%d", logEntry.ID), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get log: %d", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/sync-jobs/%d", job.ID), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete job: %d body=%s", w.Code, w.Body.String())
	}

	var leftover int64
	if err := gdb.Model(&models.SyncLog{}).Where("sync_job_id = ?", job.ID).Count(&leftover).Error; err != nil {
		t.Fatal(err)
	}
	if leftover != 0 {
		t.Fatalf("expected sync logs cascaded on job delete, got %d", leftover)
	}
}

func TestNestedRelationTreeCRUD(t *testing.T) {
	gdb := setupTestDB(t)
	r := setupRouter(t, gdb, connectors.NewRegistry())

	src := models.Connection{
		Name:   "src-nested",
		Type:   models.ConnectionTypePostgres,
		Config: datatypes.JSON([]byte(`{"host":"localhost","port":5432,"user":"u","password":"p","database":"db","sslmode":"disable"}`)),
	}
	dst := models.Connection{
		Name:   "dst-nested",
		Type:   models.ConnectionTypeTypesense,
		Config: datatypes.JSON([]byte(`{"host":"localhost","port":8108,"api_key":"xyz"}`)),
	}
	if err := gdb.Create(&src).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&dst).Error; err != nil {
		t.Fatal(err)
	}

	body := map[string]any{
		"name":                      "users-nested-tree",
		"source_connection_id":      src.ID,
		"source_table":              "users",
		"destination_connection_id": dst.ID,
		"destination_table":         "users",
		"relations": []map[string]any{
			{
				"name":  "posts",
				"type":  "has_many",
				"table": "posts",
				"fields": []map[string]any{
					{"source_name": "title", "destination_name": "title", "destination_type": "string"},
				},
				"relations": []map[string]any{
					{
						"name":  "comments",
						"type":  "has_many",
						"table": "comments",
						"relations": []map[string]any{
							{"name": "reactions", "type": "has_many", "table": "reactions"},
						},
					},
				},
			},
		},
	}
	payload, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sync-jobs", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d body=%s", w.Code, w.Body.String())
	}

	var job models.SyncJob
	if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if len(job.Relations) != 1 || job.Relations[0].Name != "posts" {
		t.Fatalf("root relations: %+v", job.Relations)
	}
	posts := job.Relations[0]
	if len(posts.Fields) != 1 || posts.Fields[0].SourceName != "title" {
		t.Fatalf("posts fields: %+v", posts.Fields)
	}
	if len(posts.Relations) != 1 || posts.Relations[0].Name != "comments" {
		t.Fatalf("comments under posts: %+v", posts.Relations)
	}
	comments := posts.Relations[0]
	if len(comments.Relations) != 1 || comments.Relations[0].Name != "reactions" {
		t.Fatalf("reactions under comments: %+v", comments.Relations)
	}

	// parent_id is internal — must not appear in API JSON
	var raw map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	rels, _ := raw["relations"].([]any)
	if len(rels) != 1 {
		t.Fatalf("raw relations: %+v", raw["relations"])
	}
	root, _ := rels[0].(map[string]any)
	if _, hasParent := root["parent_id"]; hasParent {
		t.Fatalf("parent_id should be omitted from API JSON: %+v", root)
	}
	if _, hasParent := root["parent"]; hasParent {
		t.Fatalf("parent should be omitted from API JSON: %+v", root)
	}

	// DB still stores parent_id flat
	var flat []models.SyncJobRelation
	if err := gdb.Where("sync_job_id = ?", job.ID).Find(&flat).Error; err != nil {
		t.Fatal(err)
	}
	if len(flat) != 3 {
		t.Fatalf("expected 3 flat rows, got %d", len(flat))
	}
	byName := map[string]models.SyncJobRelation{}
	for _, rel := range flat {
		byName[rel.Name] = rel
	}
	if byName["posts"].ParentID != nil {
		t.Fatalf("posts should be root, got parent_id=%v", byName["posts"].ParentID)
	}
	if byName["comments"].ParentID == nil || *byName["comments"].ParentID != byName["posts"].ID {
		t.Fatalf("comments parent: %+v", byName["comments"])
	}
	if byName["reactions"].ParentID == nil || *byName["reactions"].ParentID != byName["comments"].ID {
		t.Fatalf("reactions parent: %+v", byName["reactions"])
	}
}

type mockSource struct {
	schema    *connectors.TableSchema
	rows      []map[string]any
	schemas   map[string]*connectors.TableSchema
	tableRows map[string][]map[string]any
	readHold  <-chan struct{}
	held      atomic.Bool
}

type failingSource struct {
	err error
}

func (m *failingSource) Open(ctx context.Context) error { return m.err }
func (m *failingSource) Close() error                   { return nil }
func (m *failingSource) ListTables(ctx context.Context) ([]string, error) {
	return nil, m.err
}
func (m *failingSource) Schema(ctx context.Context, table string) (*connectors.TableSchema, error) {
	return nil, m.err
}
func (m *failingSource) Count(ctx context.Context, table string, filters []connectors.Filter) (int64, error) {
	return 0, m.err
}
func (m *failingSource) ReadChunks(ctx context.Context, table string, chunkSize int, filters []connectors.Filter, fn func([]map[string]any) error) error {
	return m.err
}
func (m *failingSource) Query(ctx context.Context, table string, columns []string, filters []connectors.Filter, limit, offset int, order *connectors.Order) ([]map[string]any, error) {
	return nil, m.err
}
func (m *failingSource) QueryRows(ctx context.Context, table string, columns []string, whereColumn string, whereValues []any) ([]map[string]any, error) {
	return nil, m.err
}

func (m *mockSource) Open(ctx context.Context) error { return nil }
func (m *mockSource) Close() error                   { return nil }
func (m *mockSource) ListTables(ctx context.Context) ([]string, error) {
	if m.schemas != nil {
		tables := make([]string, 0, len(m.schemas))
		for name := range m.schemas {
			tables = append(tables, name)
		}
		sort.Strings(tables)
		return tables, nil
	}
	if m.schema != nil {
		return []string{"main"}, nil
	}
	return nil, nil
}
func (m *mockSource) Schema(ctx context.Context, table string) (*connectors.TableSchema, error) {
	if m.schemas != nil {
		if s, ok := m.schemas[table]; ok {
			return s, nil
		}
	}
	return m.schema, nil
}
func (m *mockSource) Count(ctx context.Context, table string, filters []connectors.Filter) (int64, error) {
	n := int64(0)
	for _, row := range m.rows {
		if matchFilters(row, filters) {
			n++
		}
	}
	return n, nil
}
func (m *mockSource) ReadChunks(ctx context.Context, table string, chunkSize int, filters []connectors.Filter, fn func([]map[string]any) error) error {
	if m.readHold != nil {
		m.held.Store(true)
		select {
		case <-m.readHold:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	var filtered []map[string]any
	for _, row := range m.rows {
		if matchFilters(row, filters) {
			filtered = append(filtered, row)
		}
	}
	for i := 0; i < len(filtered); i += chunkSize {
		end := i + chunkSize
		if end > len(filtered) {
			end = len(filtered)
		}
		if err := fn(filtered[i:end]); err != nil {
			return err
		}
	}
	return nil
}
func (m *mockSource) Query(ctx context.Context, table string, columns []string, filters []connectors.Filter, limit, offset int, order *connectors.Order) ([]map[string]any, error) {
	var filtered []map[string]any
	for _, row := range m.rows {
		if matchFilters(row, filters) {
			copied := make(map[string]any, len(row))
			if len(columns) == 0 {
				for k, val := range row {
					copied[k] = val
				}
			} else {
				for _, c := range columns {
					if val, ok := row[c]; ok {
						copied[c] = val
					}
				}
			}
			filtered = append(filtered, copied)
		}
	}
	if order != nil && order.Column != "" {
		col := order.Column
		sort.SliceStable(filtered, func(i, j int) bool {
			a := fmt.Sprint(filtered[i][col])
			b := fmt.Sprint(filtered[j][col])
			if order.Desc {
				return a > b
			}
			return a < b
		})
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(filtered) {
		return []map[string]any{}, nil
	}
	if limit <= 0 {
		limit = 50
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[offset:end], nil
}
func (m *mockSource) QueryRows(ctx context.Context, table string, columns []string, whereColumn string, whereValues []any) ([]map[string]any, error) {
	if m.tableRows == nil {
		return nil, nil
	}
	rows := m.tableRows[table]
	want := make(map[string]struct{}, len(whereValues))
	for _, v := range whereValues {
		want[fmt.Sprint(v)] = struct{}{}
	}
	var out []map[string]any
	for _, row := range rows {
		v, ok := row[whereColumn]
		if !ok {
			continue
		}
		if _, ok := want[fmt.Sprint(v)]; !ok {
			continue
		}
		copied := make(map[string]any, len(row))
		if len(columns) == 0 {
			for k, val := range row {
				copied[k] = val
			}
		} else {
			for _, c := range columns {
				if val, ok := row[c]; ok {
					copied[c] = val
				}
			}
		}
		out = append(out, copied)
	}
	return out, nil
}

type mockDest struct {
	mu       sync.Mutex
	prepared bool
	batches  [][]map[string]any
	config   json.RawMessage
	onWrite  func([]map[string]any)
	docs     []map[string]any // for DestinationReader.Query in explore tests
}

func (m *mockDest) Open(ctx context.Context) error { return nil }
func (m *mockDest) Close() error                   { return nil }
func (m *mockDest) Prepare(ctx context.Context, name string, schema *connectors.TableSchema, config json.RawMessage) error {
	m.prepared = true
	m.config = append(json.RawMessage(nil), config...)
	return nil
}
func (m *mockDest) WriteBatch(ctx context.Context, name string, docs []map[string]any) error {
	copied := make([]map[string]any, len(docs))
	copy(copied, docs)
	m.mu.Lock()
	m.batches = append(m.batches, copied)
	onWrite := m.onWrite
	m.mu.Unlock()
	if onWrite != nil {
		onWrite(copied)
	}
	return nil
}
func (m *mockDest) Query(ctx context.Context, name string, filters []connectors.Filter, limit, offset int, order *connectors.Order) ([]map[string]any, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	docs := m.docs
	if docs == nil {
		var flat []map[string]any
		for _, batch := range m.batches {
			flat = append(flat, batch...)
		}
		docs = flat
	}
	var filtered []map[string]any
	for _, row := range docs {
		if matchFilters(row, filters) {
			copied := make(map[string]any, len(row))
			for k, v := range row {
				copied[k] = v
			}
			filtered = append(filtered, copied)
		}
	}
	total := int64(len(filtered))
	ordered := filtered
	if order != nil && order.Column != "" {
		ordered = make([]map[string]any, len(filtered))
		copy(ordered, filtered)
		col := order.Column
		sort.SliceStable(ordered, func(i, j int) bool {
			a := fmt.Sprint(ordered[i][col])
			b := fmt.Sprint(ordered[j][col])
			if order.Desc {
				return a > b
			}
			return a < b
		})
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(ordered) {
		return []map[string]any{}, total, nil
	}
	if limit <= 0 {
		limit = 50
	}
	end := offset + limit
	if end > len(ordered) {
		end = len(ordered)
	}
	out := make([]map[string]any, 0, end-offset)
	for _, row := range ordered[offset:end] {
		copied := make(map[string]any, len(row))
		for k, v := range row {
			copied[k] = v
		}
		out = append(out, copied)
	}
	return out, total, nil
}

func TestSyncRunWithMocks(t *testing.T) {
	gdb := setupTestDB(t)

	srcConn := models.Connection{
		Name:   "src",
		Type:   "mock_src",
		Config: datatypes.JSON([]byte(`{}`)),
	}
	dstConn := models.Connection{
		Name:   "dst",
		Type:   "mock_dst",
		Config: datatypes.JSON([]byte(`{}`)),
	}
	if err := gdb.Create(&srcConn).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&dstConn).Error; err != nil {
		t.Fatal(err)
	}
	job := models.SyncJob{
		Name:                    "test-sync",
		SourceConnectionID:      srcConn.ID,
		DestinationConnectionID: dstConn.ID,
		SourceTable:             "applicants",
		DestinationTable:        "applicants",
		ChunkSize:               2,
		Workers:                 2,
		Config:                  datatypes.JSON([]byte(`{"enable_nested_fields":false}`)),
	}
	if err := gdb.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	src := &mockSource{
		schema: &connectors.TableSchema{
			Columns: []connectors.ColumnSchema{
				{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
				{Name: "name", Type: connectors.FieldTypeString},
				{Name: "meta", Type: connectors.FieldTypeObject},
			},
		},
		rows: []map[string]any{
			{"id": 1, "name": "a", "meta": map[string]any{"x": 1}},
			{"id": 2, "name": "b", "meta": map[string]any{"x": 2}},
			{"id": 3, "name": "c", "meta": map[string]any{"x": 3}},
		},
	}
	dst := &mockDest{}

	registry := connectors.NewRegistry()
	registry.RegisterSource("mock_src", func(conn *models.Connection) (connectors.SourceReader, error) {
		return src, nil
	})
	registry.RegisterDestination("mock_dst", func(conn *models.Connection) (connectors.DestinationWriter, error) {
		return dst, nil
	})

	r := setupRouter(t, gdb, registry)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sync-jobs/%d/run", job.ID), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("run: expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var logEntry models.SyncLog
	if err := json.Unmarshal(w.Body.Bytes(), &logEntry); err != nil {
		t.Fatal(err)
	}
	if logEntry.Status != models.SyncLogStatusSuccess {
		t.Fatalf("expected success, got %s (%s)", logEntry.Status, logEntry.Message)
	}
	if logEntry.RowsTotal == nil || *logEntry.RowsTotal != 3 {
		t.Fatalf("expected rows_total=3, got %+v", logEntry.RowsTotal)
	}
	if logEntry.RowsSynced == nil || *logEntry.RowsSynced != 3 {
		t.Fatalf("expected rows_synced=3, got %+v", logEntry.RowsSynced)
	}
	if logEntry.DurationMs == nil {
		t.Fatal("expected duration_ms to be set")
	}
	if !dst.prepared {
		t.Fatal("expected destination Prepare to be called")
	}
	var got map[string]any
	if err := json.Unmarshal(dst.config, &got); err != nil {
		t.Fatalf("decode prepare config: %v", err)
	}
	if got["enable_nested_fields"] != false {
		t.Fatalf("expected enable_nested_fields=false, got %+v", got["enable_nested_fields"])
	}
	if _, ok := got["default_sorting_field"]; ok {
		t.Fatalf("expected no default_sorting_field, got %+v", got)
	}
	if len(dst.batches) == 0 {
		t.Fatal("expected batches to be written")
	}
}

func TestSyncRunWithRules(t *testing.T) {
	gdb := setupTestDB(t)

	srcConn := models.Connection{
		Name:   "src",
		Type:   "mock_src",
		Config: datatypes.JSON([]byte(`{}`)),
	}
	dstConn := models.Connection{
		Name:   "dst",
		Type:   "mock_dst",
		Config: datatypes.JSON([]byte(`{}`)),
	}
	if err := gdb.Create(&srcConn).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&dstConn).Error; err != nil {
		t.Fatal(err)
	}
	job := models.SyncJob{
		Name:                    "filtered-sync",
		SourceConnectionID:      srcConn.ID,
		DestinationConnectionID: dstConn.ID,
		SourceTable:             "users",
		DestinationTable:        "users",
		ChunkSize:               10,
		Workers:                 1,
	}
	if err := gdb.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	active := true
	inactive := false
	if err := gdb.Create(&models.SyncJobRule{
		SyncJobID: job.ID,
		Field:     "client_id",
		Operator:  models.RuleOperatorEq,
		Value:     "123",
		Active:    &active,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&models.SyncJobRule{
		SyncJobID: job.ID,
		Field:     "status",
		Operator:  models.RuleOperatorEq,
		Value:     "skip",
		Active:    &inactive,
	}).Error; err != nil {
		t.Fatal(err)
	}

	src := &mockSource{
		schema: &connectors.TableSchema{
			Columns: []connectors.ColumnSchema{
				{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
				{Name: "client_id", Type: connectors.FieldTypeInt64},
				{Name: "name", Type: connectors.FieldTypeString},
			},
		},
		rows: []map[string]any{
			{"id": 1, "client_id": int64(123), "name": "keep"},
			{"id": 2, "client_id": int64(999), "name": "drop"},
			{"id": 3, "client_id": int64(123), "name": "keep2"},
		},
	}
	dst := &mockDest{}
	registry := connectors.NewRegistry()
	registry.RegisterSource("mock_src", func(conn *models.Connection) (connectors.SourceReader, error) {
		return src, nil
	})
	registry.RegisterDestination("mock_dst", func(conn *models.Connection) (connectors.DestinationWriter, error) {
		return dst, nil
	})

	r := setupRouter(t, gdb, registry)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sync-jobs/%d/run", job.ID), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("run: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var logEntry models.SyncLog
	if err := json.Unmarshal(w.Body.Bytes(), &logEntry); err != nil {
		t.Fatal(err)
	}
	if logEntry.Status != models.SyncLogStatusSuccess {
		t.Fatalf("expected success, got %s (%s)", logEntry.Status, logEntry.Message)
	}
	if logEntry.RowsTotal == nil || *logEntry.RowsTotal != 2 {
		t.Fatalf("expected rows_total=2, got %+v", logEntry.RowsTotal)
	}
	if logEntry.RowsSynced == nil || *logEntry.RowsSynced != 2 {
		t.Fatalf("expected rows_synced=2, got %+v", logEntry.RowsSynced)
	}
	var names []string
	for _, batch := range dst.batches {
		for _, doc := range batch {
			names = append(names, fmt.Sprint(doc["name"]))
		}
	}
	sort.Strings(names)
	if len(names) != 2 || names[0] != "keep" || names[1] != "keep2" {
		t.Fatalf("unexpected synced names: %v", names)
	}
}

func TestSyncStartReturnsRunningThenCompletes(t *testing.T) {
	gdb := setupTestDB(t)

	srcConn := models.Connection{
		Name:   "src",
		Type:   "mock_src",
		Config: datatypes.JSON([]byte(`{}`)),
	}
	dstConn := models.Connection{
		Name:   "dst",
		Type:   "mock_dst",
		Config: datatypes.JSON([]byte(`{}`)),
	}
	if err := gdb.Create(&srcConn).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&dstConn).Error; err != nil {
		t.Fatal(err)
	}
	job := models.SyncJob{
		Name:                    "test-start",
		SourceConnectionID:      srcConn.ID,
		DestinationConnectionID: dstConn.ID,
		SourceTable:             "applicants",
		DestinationTable:        "applicants",
		ChunkSize:               2,
		Workers:                 1,
		Config:                  datatypes.JSON([]byte(`{}`)),
	}
	if err := gdb.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	src := &mockSource{
		schema: &connectors.TableSchema{
			Columns: []connectors.ColumnSchema{
				{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
				{Name: "name", Type: connectors.FieldTypeString},
			},
		},
		rows: []map[string]any{
			{"id": 1, "name": "a"},
			{"id": 2, "name": "b"},
		},
	}
	dst := &mockDest{}

	registry := connectors.NewRegistry()
	registry.RegisterSource("mock_src", func(conn *models.Connection) (connectors.SourceReader, error) {
		return src, nil
	})
	registry.RegisterDestination("mock_dst", func(conn *models.Connection) (connectors.DestinationWriter, error) {
		return dst, nil
	})

	r := setupRouter(t, gdb, registry)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sync-jobs/%d/start", job.ID), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("start: expected 202, got %d body=%s", w.Code, w.Body.String())
	}

	var started models.SyncLog
	if err := json.Unmarshal(w.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	if started.Status != models.SyncLogStatusRunning {
		t.Fatalf("expected running, got %s", started.Status)
	}
	if started.ID == 0 {
		t.Fatal("expected sync log id")
	}

	deadline := time.Now().Add(5 * time.Second)
	var finished models.SyncLog
	for {
		gw := httptest.NewRecorder()
		greq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sync-logs/%d", started.ID), nil)
		r.ServeHTTP(gw, greq)
		if gw.Code != http.StatusOK {
			t.Fatalf("get log: expected 200, got %d body=%s", gw.Code, gw.Body.String())
		}
		if err := json.Unmarshal(gw.Body.Bytes(), &finished); err != nil {
			t.Fatal(err)
		}
		if finished.Status != models.SyncLogStatusRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for background sync to finish")
		}
		time.Sleep(20 * time.Millisecond)
	}

	if finished.Status != models.SyncLogStatusSuccess {
		t.Fatalf("expected success, got %s (%s)", finished.Status, finished.Message)
	}
	if finished.RowsSynced == nil || *finished.RowsSynced != 2 {
		t.Fatalf("expected rows_synced=2, got %+v", finished.RowsSynced)
	}
}

func TestSyncStopCancelsRunningJob(t *testing.T) {
	gdb := setupTestDB(t)

	srcConn := models.Connection{
		Name:   "src",
		Type:   "mock_src",
		Config: datatypes.JSON([]byte(`{}`)),
	}
	dstConn := models.Connection{
		Name:   "dst",
		Type:   "mock_dst",
		Config: datatypes.JSON([]byte(`{}`)),
	}
	if err := gdb.Create(&srcConn).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&dstConn).Error; err != nil {
		t.Fatal(err)
	}
	job := models.SyncJob{
		Name:                    "test-stop",
		SourceConnectionID:      srcConn.ID,
		DestinationConnectionID: dstConn.ID,
		SourceTable:             "applicants",
		DestinationTable:        "applicants",
		ChunkSize:               1,
		Workers:                 1,
		Config:                  datatypes.JSON([]byte(`{}`)),
	}
	if err := gdb.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	gate := make(chan struct{})
	src := &mockSource{
		schema: &connectors.TableSchema{
			Columns: []connectors.ColumnSchema{
				{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
				{Name: "name", Type: connectors.FieldTypeString},
			},
		},
		rows: []map[string]any{
			{"id": 1, "name": "a"},
			{"id": 2, "name": "b"},
		},
		readHold: gate,
	}
	dst := &mockDest{}

	registry := connectors.NewRegistry()
	registry.RegisterSource("mock_src", func(conn *models.Connection) (connectors.SourceReader, error) {
		return src, nil
	})
	registry.RegisterDestination("mock_dst", func(conn *models.Connection) (connectors.DestinationWriter, error) {
		return dst, nil
	})

	r := setupRouter(t, gdb, registry)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sync-jobs/%d/start", job.ID), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("start: expected 202, got %d body=%s", w.Code, w.Body.String())
	}

	var started models.SyncLog
	if err := json.Unmarshal(w.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for !src.held.Load() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for sync to block")
		}
		time.Sleep(5 * time.Millisecond)
	}

	sw := httptest.NewRecorder()
	sreq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sync-logs/%d/stop", started.ID), nil)
	r.ServeHTTP(sw, sreq)
	if sw.Code != http.StatusOK {
		t.Fatalf("stop: expected 200, got %d body=%s", sw.Code, sw.Body.String())
	}

	var stopped models.SyncLog
	if err := json.Unmarshal(sw.Body.Bytes(), &stopped); err != nil {
		t.Fatal(err)
	}
	if stopped.Status != models.SyncLogStatusStopped {
		t.Fatalf("expected stopped, got %s (%s)", stopped.Status, stopped.Message)
	}

	close(gate)
	deadline = time.Now().Add(2 * time.Second)
	for {
		var current models.SyncLog
		if err := gdb.First(&current, started.ID).Error; err == nil && current.FinishedAt != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for stopped sync to settle")
		}
		time.Sleep(10 * time.Millisecond)
	}

	cw := httptest.NewRecorder()
	creq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sync-logs/%d/stop", started.ID), nil)
	r.ServeHTTP(cw, creq)
	if cw.Code != http.StatusConflict {
		t.Fatalf("stop again: expected 409, got %d body=%s", cw.Code, cw.Body.String())
	}
}

func TestSyncRunUpdatesRowsSyncedAfterEachChunk(t *testing.T) {
	gdb := setupTestDB(t)

	srcConn := models.Connection{
		Name:   "src",
		Type:   "mock_src",
		Config: datatypes.JSON([]byte(`{}`)),
	}
	dstConn := models.Connection{
		Name:   "dst",
		Type:   "mock_dst",
		Config: datatypes.JSON([]byte(`{}`)),
	}
	if err := gdb.Create(&srcConn).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&dstConn).Error; err != nil {
		t.Fatal(err)
	}
	job := models.SyncJob{
		Name:                    "chunk-progress",
		SourceConnectionID:      srcConn.ID,
		DestinationConnectionID: dstConn.ID,
		SourceTable:             "applicants",
		DestinationTable:        "applicants",
		ChunkSize:               2,
		Workers:                 1,
		Config:                  datatypes.JSON([]byte(`{}`)),
	}
	if err := gdb.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	src := &mockSource{
		schema: &connectors.TableSchema{
			Columns: []connectors.ColumnSchema{
				{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
				{Name: "name", Type: connectors.FieldTypeString},
			},
		},
		rows: []map[string]any{
			{"id": 1, "name": "a"},
			{"id": 2, "name": "b"},
			{"id": 3, "name": "c"},
			{"id": 4, "name": "d"},
		},
	}

	var snapshots []int64
	var totals []int64
	var durations []*int64
	dst := &mockDest{
		onWrite: func([]map[string]any) {
			var logEntry models.SyncLog
			if err := gdb.Order("id desc").First(&logEntry).Error; err != nil {
				t.Errorf("load sync log: %v", err)
				return
			}
			var synced int64
			if logEntry.RowsSynced != nil {
				synced = *logEntry.RowsSynced
			}
			var total int64
			if logEntry.RowsTotal != nil {
				total = *logEntry.RowsTotal
			}
			snapshots = append(snapshots, synced)
			totals = append(totals, total)
			durations = append(durations, logEntry.DurationMs)
		},
	}

	registry := connectors.NewRegistry()
	registry.RegisterSource("mock_src", func(conn *models.Connection) (connectors.SourceReader, error) {
		return src, nil
	})
	registry.RegisterDestination("mock_dst", func(conn *models.Connection) (connectors.DestinationWriter, error) {
		return dst, nil
	})

	r := setupRouter(t, gdb, registry)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sync-jobs/%d/run", job.ID), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("run: expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	if len(snapshots) != 2 {
		t.Fatalf("expected 2 chunk snapshots, got %v", snapshots)
	}
	if snapshots[0] != 0 {
		t.Fatalf("expected rows_synced=0 before the first chunk was recorded, got %d", snapshots[0])
	}
	if snapshots[1] != 2 {
		t.Fatalf("expected rows_synced=2 after the first chunk, got %d", snapshots[1])
	}
	if totals[0] != 4 || totals[1] != 4 {
		t.Fatalf("expected rows_total=4 while chunks were writing, got %v", totals)
	}
	if durations[0] != nil {
		t.Fatalf("expected duration_ms unset before the first chunk was recorded, got %v", *durations[0])
	}
	if durations[1] == nil {
		t.Fatal("expected duration_ms to be set after the first chunk")
	}

	var logEntry models.SyncLog
	if err := json.Unmarshal(w.Body.Bytes(), &logEntry); err != nil {
		t.Fatal(err)
	}
	if logEntry.RowsSynced == nil || *logEntry.RowsSynced != 4 {
		t.Fatalf("expected final rows_synced=4, got %+v", logEntry.RowsSynced)
	}
	if logEntry.DurationMs == nil {
		t.Fatal("expected final duration_ms to be set")
	}
}

func TestSyncRunWithRelations(t *testing.T) {
	gdb := setupTestDB(t)

	srcConn := models.Connection{
		Name:   "src",
		Type:   "mock_src",
		Config: datatypes.JSON([]byte(`{}`)),
	}
	dstConn := models.Connection{
		Name:   "dst",
		Type:   "mock_dst",
		Config: datatypes.JSON([]byte(`{}`)),
	}
	if err := gdb.Create(&srcConn).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&dstConn).Error; err != nil {
		t.Fatal(err)
	}
	job := models.SyncJob{
		Name:                    "applicants-with-tags",
		SourceConnectionID:      srcConn.ID,
		DestinationConnectionID: dstConn.ID,
		SourceTable:             "applicants",
		DestinationTable:        "applicants",
		ChunkSize:               10,
		Workers:                 1,
		Relations: []models.SyncJobRelation{
			{
				Name:   "tags",
				Type:   models.RelationTypeBelongsToMany,
				Table:  "tags",
				Config: datatypes.JSON([]byte(`{"pivot_table":"applicant_tags"}`)),
			},
		},
	}
	if err := gdb.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	src := &mockSource{
		schema: &connectors.TableSchema{
			Columns: []connectors.ColumnSchema{
				{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
				{Name: "name", Type: connectors.FieldTypeString},
			},
		},
		schemas: map[string]*connectors.TableSchema{
			"applicants": {
				Columns: []connectors.ColumnSchema{
					{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
					{Name: "name", Type: connectors.FieldTypeString},
				},
			},
			"tags": {
				Columns: []connectors.ColumnSchema{
					{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
					{Name: "name", Type: connectors.FieldTypeString},
				},
			},
		},
		rows: []map[string]any{
			{"id": 1, "name": "Ada"},
			{"id": 2, "name": "Bob"},
		},
		tableRows: map[string][]map[string]any{
			"applicant_tags": {
				{"applicant_id": 1, "tag_id": 10},
				{"applicant_id": 1, "tag_id": 11},
				{"applicant_id": 2, "tag_id": 10},
			},
			"tags": {
				{"id": 10, "name": "vip"},
				{"id": 11, "name": "referral"},
			},
		},
	}
	dst := &mockDest{}

	registry := connectors.NewRegistry()
	registry.RegisterSource("mock_src", func(conn *models.Connection) (connectors.SourceReader, error) {
		return src, nil
	})
	registry.RegisterDestination("mock_dst", func(conn *models.Connection) (connectors.DestinationWriter, error) {
		return dst, nil
	})

	r := setupRouter(t, gdb, registry)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sync-jobs/%d/run", job.ID), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("run: expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var logEntry models.SyncLog
	if err := json.Unmarshal(w.Body.Bytes(), &logEntry); err != nil {
		t.Fatal(err)
	}
	if logEntry.Status != models.SyncLogStatusSuccess {
		t.Fatalf("expected success, got %s (%s)", logEntry.Status, logEntry.Message)
	}

	var allDocs []map[string]any
	for _, batch := range dst.batches {
		allDocs = append(allDocs, batch...)
	}
	if len(allDocs) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(allDocs))
	}

	byID := map[string]map[string]any{}
	for _, doc := range allDocs {
		byID[fmt.Sprint(doc["id"])] = doc
	}
	adaTags, ok := byID["1"]["tags"].([]map[string]any)
	if !ok || len(adaTags) != 2 {
		t.Fatalf("expected Ada to have 2 tags, got %#v", byID["1"]["tags"])
	}
	names := map[string]bool{}
	for _, tag := range adaTags {
		names[fmt.Sprint(tag["name"])] = true
	}
	if !names["vip"] || !names["referral"] {
		t.Fatalf("unexpected Ada tags: %#v", adaTags)
	}
	bobTags, ok := byID["2"]["tags"].([]map[string]any)
	if !ok || len(bobTags) != 1 || fmt.Sprint(bobTags[0]["name"]) != "vip" {
		t.Fatalf("expected Bob to have vip tag, got %#v", byID["2"]["tags"])
	}
}

func TestSyncRunWithNestedHasMany(t *testing.T) {
	gdb := setupTestDB(t)

	srcConn := models.Connection{
		Name:   "src",
		Type:   "mock_src",
		Config: datatypes.JSON([]byte(`{}`)),
	}
	dstConn := models.Connection{
		Name:   "dst",
		Type:   "mock_dst",
		Config: datatypes.JSON([]byte(`{}`)),
	}
	if err := gdb.Create(&srcConn).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&dstConn).Error; err != nil {
		t.Fatal(err)
	}
	job := models.SyncJob{
		Name:                    "users-nested",
		SourceConnectionID:      srcConn.ID,
		DestinationConnectionID: dstConn.ID,
		SourceTable:             "users",
		DestinationTable:        "users",
		ChunkSize:               10,
		Workers:                 1,
	}
	if err := gdb.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	postsRel := models.SyncJobRelation{
		SyncJobID:  job.ID,
		Name:       "posts",
		Type:       models.RelationTypeHasMany,
		Table:      "posts",
		ForeignKey: "user_id",
	}
	if err := gdb.Create(&postsRel).Error; err != nil {
		t.Fatal(err)
	}
	commentsRel := models.SyncJobRelation{
		SyncJobID:  job.ID,
		ParentID:   &postsRel.ID,
		Name:       "comments",
		Type:       models.RelationTypeHasMany,
		Table:      "comments",
		ForeignKey: "post_id",
	}
	if err := gdb.Create(&commentsRel).Error; err != nil {
		t.Fatal(err)
	}
	reactionsRel := models.SyncJobRelation{
		SyncJobID:  job.ID,
		ParentID:   &commentsRel.ID,
		Name:       "reactions",
		Type:       models.RelationTypeHasMany,
		Table:      "reactions",
		ForeignKey: "comment_id",
	}
	if err := gdb.Create(&reactionsRel).Error; err != nil {
		t.Fatal(err)
	}

	src := &mockSource{
		schema: &connectors.TableSchema{
			Columns: []connectors.ColumnSchema{
				{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
				{Name: "username", Type: connectors.FieldTypeString},
			},
		},
		schemas: map[string]*connectors.TableSchema{
			"users": {
				Columns: []connectors.ColumnSchema{
					{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
					{Name: "username", Type: connectors.FieldTypeString},
				},
			},
			"posts": {
				Columns: []connectors.ColumnSchema{
					{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
					{Name: "user_id", Type: connectors.FieldTypeInt64},
					{Name: "title", Type: connectors.FieldTypeString},
				},
			},
			"comments": {
				Columns: []connectors.ColumnSchema{
					{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
					{Name: "post_id", Type: connectors.FieldTypeInt64},
					{Name: "body", Type: connectors.FieldTypeString},
				},
			},
			"reactions": {
				Columns: []connectors.ColumnSchema{
					{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
					{Name: "comment_id", Type: connectors.FieldTypeInt64},
					{Name: "type", Type: connectors.FieldTypeString},
				},
			},
		},
		rows: []map[string]any{
			{"id": 1, "username": "ada"},
			{"id": 2, "username": "bob"},
		},
		tableRows: map[string][]map[string]any{
			"posts": {
				{"id": 10, "user_id": 1, "title": "hello"},
				{"id": 11, "user_id": 1, "title": "world"},
				{"id": 20, "user_id": 2, "title": "bob-post"},
			},
			"comments": {
				{"id": 100, "post_id": 10, "body": "nice"},
				{"id": 101, "post_id": 10, "body": "cool"},
				{"id": 200, "post_id": 20, "body": "hi"},
			},
			"reactions": {
				{"id": 1000, "comment_id": 100, "type": "like"},
				{"id": 1001, "comment_id": 100, "type": "smile"},
			},
		},
	}
	dst := &mockDest{}

	registry := connectors.NewRegistry()
	registry.RegisterSource("mock_src", func(conn *models.Connection) (connectors.SourceReader, error) {
		return src, nil
	})
	registry.RegisterDestination("mock_dst", func(conn *models.Connection) (connectors.DestinationWriter, error) {
		return dst, nil
	})

	r := setupRouter(t, gdb, registry)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sync-jobs/%d/run", job.ID), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("run: expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var logEntry models.SyncLog
	if err := json.Unmarshal(w.Body.Bytes(), &logEntry); err != nil {
		t.Fatal(err)
	}
	if logEntry.Status != models.SyncLogStatusSuccess {
		t.Fatalf("expected success, got %s (%s)", logEntry.Status, logEntry.Message)
	}

	var allDocs []map[string]any
	for _, batch := range dst.batches {
		allDocs = append(allDocs, batch...)
	}
	if len(allDocs) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(allDocs))
	}

	byID := map[string]map[string]any{}
	for _, doc := range allDocs {
		byID[fmt.Sprint(doc["id"])] = doc
	}

	adaPosts, ok := byID["1"]["posts"].([]map[string]any)
	if !ok || len(adaPosts) != 2 {
		t.Fatalf("expected Ada 2 posts, got %#v", byID["1"]["posts"])
	}
	var hello map[string]any
	for _, p := range adaPosts {
		if fmt.Sprint(p["title"]) == "hello" {
			hello = p
		}
	}
	if hello == nil {
		t.Fatal("expected Ada post titled hello")
	}
	comments, ok := hello["comments"].([]map[string]any)
	if !ok || len(comments) != 2 {
		t.Fatalf("expected 2 comments on hello, got %#v", hello["comments"])
	}
	var nice map[string]any
	for _, c := range comments {
		if fmt.Sprint(c["body"]) == "nice" {
			nice = c
		}
	}
	if nice == nil {
		t.Fatal("expected nice comment")
	}
	reactions, ok := nice["reactions"].([]map[string]any)
	if !ok || len(reactions) != 2 {
		t.Fatalf("expected 2 reactions on nice, got %#v", nice["reactions"])
	}

	bobPosts, ok := byID["2"]["posts"].([]map[string]any)
	if !ok || len(bobPosts) != 1 {
		t.Fatalf("expected Bob 1 post, got %#v", byID["2"]["posts"])
	}
	bobComments, ok := bobPosts[0]["comments"].([]map[string]any)
	if !ok || len(bobComments) != 1 {
		t.Fatalf("expected Bob 1 comment, got %#v", bobPosts[0]["comments"])
	}
	bobReactions, ok := bobComments[0]["reactions"].([]map[string]any)
	if !ok || len(bobReactions) != 0 {
		t.Fatalf("expected Bob comment 0 reactions, got %#v", bobComments[0]["reactions"])
	}
}

func TestSyncRunWithBelongsToAndRelationFields(t *testing.T) {
	gdb := setupTestDB(t)

	srcConn := models.Connection{
		Name: "src", Type: "mock_src", Config: datatypes.JSON([]byte(`{}`)),
	}
	dstConn := models.Connection{
		Name: "dst", Type: "mock_dst", Config: datatypes.JSON([]byte(`{}`)),
	}
	if err := gdb.Create(&srcConn).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&dstConn).Error; err != nil {
		t.Fatal(err)
	}
	job := models.SyncJob{
		Name:                    "posts-with-author",
		SourceConnectionID:      srcConn.ID,
		SourceTable:             "posts",
		DestinationConnectionID: dstConn.ID,
		DestinationTable:        "posts",
		ChunkSize:               10,
		Workers:                 1,
	}
	if err := gdb.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	authorRel := models.SyncJobRelation{
		SyncJobID:  job.ID,
		Name:       "author",
		Type:       models.RelationTypeBelongsTo,
		Table:      "users",
		ForeignKey: "user_id",
	}
	if err := gdb.Create(&authorRel).Error; err != nil {
		t.Fatal(err)
	}
	active := true
	if err := gdb.Create(&models.SyncJobField{
		SyncJobID:         job.ID,
		SyncJobRelationID: &authorRel.ID,
		SourceName:        "email",
		DestinationName:   "email_address",
		DestinationType:   "string",
		Active:            &active,
	}).Error; err != nil {
		t.Fatal(err)
	}

	src := &mockSource{
		schema: &connectors.TableSchema{
			Columns: []connectors.ColumnSchema{
				{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
				{Name: "title", Type: connectors.FieldTypeString},
				{Name: "user_id", Type: connectors.FieldTypeInt64},
			},
		},
		schemas: map[string]*connectors.TableSchema{
			"posts": {
				Columns: []connectors.ColumnSchema{
					{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
					{Name: "title", Type: connectors.FieldTypeString},
					{Name: "user_id", Type: connectors.FieldTypeInt64},
				},
			},
			"users": {
				Columns: []connectors.ColumnSchema{
					{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
					{Name: "name", Type: connectors.FieldTypeString},
					{Name: "email", Type: connectors.FieldTypeString},
				},
			},
		},
		rows: []map[string]any{
			{"id": 1, "title": "hello", "user_id": 10},
		},
		tableRows: map[string][]map[string]any{
			"users": {
				{"id": 10, "name": "Ada", "email": "ada@example.com"},
			},
		},
	}
	dst := &mockDest{}
	registry := connectors.NewRegistry()
	registry.RegisterSource("mock_src", func(conn *models.Connection) (connectors.SourceReader, error) {
		return src, nil
	})
	registry.RegisterDestination("mock_dst", func(conn *models.Connection) (connectors.DestinationWriter, error) {
		return dst, nil
	})

	r := setupRouter(t, gdb, registry)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sync-jobs/%d/run", job.ID), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("run: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var logEntry models.SyncLog
	if err := json.Unmarshal(w.Body.Bytes(), &logEntry); err != nil {
		t.Fatal(err)
	}
	if logEntry.Status != models.SyncLogStatusSuccess {
		t.Fatalf("expected success, got %s (%s)", logEntry.Status, logEntry.Message)
	}
	if len(dst.batches) == 0 || len(dst.batches[0]) == 0 {
		t.Fatal("expected written docs")
	}
	author, ok := dst.batches[0][0]["author"].(map[string]any)
	if !ok {
		t.Fatalf("expected author object, got %#v", dst.batches[0][0]["author"])
	}
	if fmt.Sprint(author["email_address"]) != "ada@example.com" {
		t.Fatalf("expected renamed email field, got %#v", author)
	}
	if _, exists := author["email"]; exists {
		t.Fatalf("expected source email removed after rename, got %#v", author)
	}
}

func TestEnsureID(t *testing.T) {
	schema := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "uid", Type: connectors.FieldTypeInt64, PrimaryKey: true},
			{Name: "name", Type: connectors.FieldTypeString},
		},
	}
	docs := []map[string]any{
		{"uid": 10, "name": "x"},
	}
	connectors.EnsureID(docs, schema, 0)
	if docs[0]["id"] != "10" {
		t.Fatalf("expected id 10, got %v", docs[0]["id"])
	}
}

func TestExploreSyncJobSourceAndExport(t *testing.T) {
	gdb := setupTestDB(t)

	srcConn := models.Connection{Name: "src", Type: "mock_src", Config: datatypes.JSON([]byte(`{}`))}
	dstConn := models.Connection{Name: "dst", Type: "mock_dst", Config: datatypes.JSON([]byte(`{}`))}
	if err := gdb.Create(&srcConn).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&dstConn).Error; err != nil {
		t.Fatal(err)
	}
	job := models.SyncJob{
		Name:                    "explore-job",
		SourceConnectionID:      srcConn.ID,
		DestinationConnectionID: dstConn.ID,
		SourceTable:             "applicants",
		DestinationTable:        "applicants",
		ChunkSize:               50,
		Workers:                 1,
	}
	if err := gdb.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	active := true
	if err := gdb.Create(&models.SyncJobField{
		SyncJobID:       job.ID,
		SourceName:      "name",
		DestinationName: "full_name",
		Active:          &active,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&models.SyncJobRule{
		SyncJobID: job.ID,
		Field:     "status",
		Operator:  models.RuleOperatorEq,
		Value:     "active",
		Active:    &active,
	}).Error; err != nil {
		t.Fatal(err)
	}

	src := &mockSource{
		schema: &connectors.TableSchema{
			Columns: []connectors.ColumnSchema{
				{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
				{Name: "name", Type: connectors.FieldTypeString},
				{Name: "status", Type: connectors.FieldTypeString},
			},
		},
		rows: []map[string]any{
			{"id": 1, "name": "a", "status": "active"},
			{"id": 2, "name": "b", "status": "inactive"},
			{"id": 3, "name": "c", "status": "active"},
		},
	}
	dst := &mockDest{
		docs: []map[string]any{
			{"id": "1", "full_name": "a"},
			{"id": "3", "full_name": "c"},
		},
	}
	registry := connectors.NewRegistry()
	registry.RegisterSource("mock_src", func(conn *models.Connection) (connectors.SourceReader, error) {
		return src, nil
	})
	registry.RegisterDestination("mock_dst", func(conn *models.Connection) (connectors.DestinationWriter, error) {
		return dst, nil
	})

	r := setupRouter(t, gdb, registry)

	body, _ := json.Marshal(map[string]any{"side": "source", "page": 1, "page_size": 10})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sync-jobs/%d/explore", job.ID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("explore source status=%d body=%s", w.Code, w.Body.String())
	}
	var preview syncsvc.PreviewResult
	if err := json.Unmarshal(w.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Total != 2 {
		t.Fatalf("expected total 2 active rows, got %d", preview.Total)
	}
	if len(preview.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(preview.Rows))
	}
	if fmt.Sprint(preview.Rows[0]["full_name"]) != "a" {
		t.Fatalf("expected renamed field full_name=a, got %#v", preview.Rows[0])
	}
	wantCols := []string{"id", "full_name", "status"}
	if len(preview.Columns) != len(wantCols) {
		t.Fatalf("expected columns %v, got %v", wantCols, preview.Columns)
	}
	for i, c := range wantCols {
		if preview.Columns[i] != c {
			t.Fatalf("expected columns %v, got %v", wantCols, preview.Columns)
		}
	}

	filterBody, _ := json.Marshal(map[string]any{
		"side":      "source",
		"page":      1,
		"page_size": 10,
		"filters": []map[string]any{
			{"field": "full_name", "operator": "eq", "value": "c"},
		},
	})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sync-jobs/%d/explore", job.ID), bytes.NewReader(filterBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("explore filter status=%d body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Total != 1 || len(preview.Rows) != 1 || fmt.Sprint(preview.Rows[0]["full_name"]) != "c" {
		t.Fatalf("expected 1 filtered row full_name=c, got total=%d rows=%#v", preview.Total, preview.Rows)
	}

	destFilterBody, _ := json.Marshal(map[string]any{
		"side":      "destination",
		"page":      1,
		"page_size": 10,
		"filters": []map[string]any{
			{"field": "full_name", "operator": "eq", "value": "a"},
		},
	})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sync-jobs/%d/explore", job.ID), bytes.NewReader(destFilterBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("explore dest filter status=%d body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Total != 1 || len(preview.Rows) != 1 || fmt.Sprint(preview.Rows[0]["full_name"]) != "a" {
		t.Fatalf("expected 1 dest filtered row, got total=%d rows=%#v", preview.Total, preview.Rows)
	}

	sortBody, _ := json.Marshal(map[string]any{
		"side":      "source",
		"page":      1,
		"page_size": 10,
		"sort_by":   "full_name",
		"sort_dir":  "desc",
	})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sync-jobs/%d/explore", job.ID), bytes.NewReader(sortBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("explore sort status=%d body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.SortBy != "full_name" || preview.SortDir != "desc" {
		t.Fatalf("expected sort full_name desc, got %q %q", preview.SortBy, preview.SortDir)
	}
	if len(preview.Rows) < 2 || fmt.Sprint(preview.Rows[0]["full_name"]) != "c" {
		t.Fatalf("expected full_name desc first row c, got %#v", preview.Rows)
	}

	exportBody, _ := json.Marshal(map[string]any{"side": "source"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sync-jobs/%d/explore/export", job.ID), bytes.NewReader(exportBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("export status=%d body=%s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if ct != "text/csv; charset=utf-8" {
		t.Fatalf("expected csv content-type, got %q", ct)
	}
	csvText := w.Body.String()
	if !bytes.Contains(w.Body.Bytes(), []byte("full_name")) {
		t.Fatalf("expected full_name header in csv, got %q", csvText)
	}

	destBody, _ := json.Marshal(map[string]any{"side": "destination", "page": 1, "page_size": 10})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sync-jobs/%d/explore", job.ID), bytes.NewReader(destBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("explore destination status=%d body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Total != 2 || len(preview.Rows) != 2 {
		t.Fatalf("expected 2 destination rows, got total=%d rows=%d", preview.Total, len(preview.Rows))
	}
	destCols := append([]string(nil), preview.Columns...)
	wantDestCols := []string{"id", "full_name", "status"}
	if !reflect.DeepEqual(destCols, wantDestCols) {
		t.Fatalf("expected destination columns %v, got %v", wantDestCols, destCols)
	}

	destSortBody, _ := json.Marshal(map[string]any{
		"side": "destination", "page": 1, "page_size": 10,
		"sort_by": "full_name", "sort_dir": "desc",
	})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sync-jobs/%d/explore", job.ID), bytes.NewReader(destSortBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("explore destination sort status=%d body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(preview.Columns, destCols) {
		t.Fatalf("destination column order changed after sort: before %v after %v", destCols, preview.Columns)
	}

	badBody, _ := json.Marshal(map[string]any{"side": "neither"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sync-jobs/%d/explore", job.ID), bytes.NewReader(badBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad side, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/sync-jobs/99999/explore", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing job, got %d body=%s", w.Code, w.Body.String())
	}
}
