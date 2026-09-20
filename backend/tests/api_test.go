package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
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
		Columns []string `json:"columns"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &columnsResp); err != nil {
		t.Fatalf("decode columns: %v", err)
	}
	if len(columnsResp.Columns) != 2 || columnsResp.Columns[0] != "id" || columnsResp.Columns[1] != "email" {
		t.Fatalf("unexpected columns: %v", columnsResp.Columns)
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
		"destination_connection_id": dst.ID,
		"source_table":              "applicants",
		"destination_table":         "applicants",
		"chunk_size":                100,
		"parallel_count":            2,
		"config": map[string]any{
			"default_sorting_field": "id",
			"enable_nested_fields":  true,
		},
		"relations": []map[string]any{
			{
				"name":        "tags",
				"type":        "belongs_to_many",
				"table":       "tags",
				"pivot_table": "applicant_tags",
			},
			{
				"name":        "skills",
				"type":        "belongs_to_many",
				"table":       "skills",
				"pivot_table": "applicant_skills",
				"active":      false,
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
	if job.Relations[0].Name != "tags" || job.Relations[0].PivotTable != "applicant_tags" || !job.Relations[0].IsActive() {
		t.Fatalf("expected active tags relation, got %+v", job.Relations[0])
	}
	if job.Relations[1].Name != "skills" || job.Relations[1].IsActive() {
		t.Fatalf("expected inactive skills relation, got %+v", job.Relations[1])
	}
	if len(job.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %+v", job.Fields)
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
	var jobCfg map[string]any
	if err := json.Unmarshal(job.Config, &jobCfg); err != nil {
		t.Fatalf("decode job config: %v", err)
	}
	if jobCfg["default_sorting_field"] != "id" {
		t.Fatalf("expected default_sorting_field=id, got %+v", jobCfg)
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
	if len(job.Fields) != 3 {
		t.Fatalf("expected preloaded fields, got %+v", job.Fields)
	}

	update := map[string]any{
		"relations": []map[string]any{
			{
				"name":        "tags",
				"type":        "belongs_to_many",
				"table":       "tags",
				"pivot_table": "applicant_tags",
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

type mockSource struct {
	schema    *connectors.TableSchema
	rows      []map[string]any
	schemas   map[string]*connectors.TableSchema
	tableRows map[string][]map[string]any
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
func (m *mockSource) Count(ctx context.Context, table string) (int64, error) {
	return int64(len(m.rows)), nil
}
func (m *mockSource) ReadChunks(ctx context.Context, table string, chunkSize int, fn func([]map[string]any) error) error {
	for i := 0; i < len(m.rows); i += chunkSize {
		end := i + chunkSize
		if end > len(m.rows) {
			end = len(m.rows)
		}
		if err := fn(m.rows[i:end]); err != nil {
			return err
		}
	}
	return nil
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
		ParallelCount:           2,
		Config:                  datatypes.JSON([]byte(`{"default_sorting_field":"id","enable_nested_fields":false}`)),
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
	if got["default_sorting_field"] != "id" {
		t.Fatalf("expected default_sorting_field=id, got %+v", got)
	}
	if got["enable_nested_fields"] != false {
		t.Fatalf("expected enable_nested_fields=false, got %+v", got["enable_nested_fields"])
	}
	if len(dst.batches) == 0 {
		t.Fatal("expected batches to be written")
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
		ParallelCount:           1,
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
		ParallelCount:           1,
		Relations: []models.SyncJobRelation{
			{
				Name:       "tags",
				Type:       models.RelationTypeBelongsToMany,
				Table:      "tags",
				PivotTable: "applicant_tags",
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
		ParallelCount:           1,
		Relations: []models.SyncJobRelation{
			{
				Name:       "posts",
				Type:       models.RelationTypeHasMany,
				Table:      "posts",
				ForeignKey: "user_id",
			},
			{
				Name:           "comments",
				Type:           models.RelationTypeHasMany,
				Table:          "comments",
				ForeignKey:     "post_id",
				ParentRelation: "posts",
			},
			{
				Name:           "reactions",
				Type:           models.RelationTypeHasMany,
				Table:          "reactions",
				ForeignKey:     "comment_id",
				ParentRelation: "comments",
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
