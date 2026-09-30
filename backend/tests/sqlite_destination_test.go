package tests

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/sqlite"
	"github.com/portico/backend/internal/models"
	"gorm.io/datatypes"
)

func TestSQLiteParseConfig(t *testing.T) {
	cfg, err := sqlite.ParseConfig([]byte(`{"path":"./data.db"}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Path != "./data.db" {
		t.Fatalf("path=%q", cfg.Path)
	}
	if _, err := sqlite.ParseConfig([]byte(`{}`)); err == nil {
		t.Fatal("expected path required")
	}
}

func TestSQLiteBuildCreateTableSQL(t *testing.T) {
	schema := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
			{Name: "name", Type: connectors.FieldTypeString},
			{Name: "posts", Type: connectors.FieldTypeObjectArray},
			{Name: "active", Type: connectors.FieldTypeBool},
			{Name: "score", Type: connectors.FieldTypeFloat64},
		},
	}
	sql, err := sqlite.BuildCreateTableSQL("users_copy", schema)
	if err != nil {
		t.Fatal(err)
	}
	wantParts := []string{
		`CREATE TABLE "users_copy"`,
		`"id" INTEGER`,
		`"name" TEXT`,
		`"posts" TEXT`,
		`"active" INTEGER`,
		`"score" REAL`,
		`PRIMARY KEY ("id")`,
	}
	for _, part := range wantParts {
		if !strings.Contains(sql, part) {
			t.Fatalf("sql missing %q:\n%s", part, sql)
		}
	}
}

func TestSQLiteRowForWrite(t *testing.T) {
	schema := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
			{Name: "posts", Type: connectors.FieldTypeObjectArray},
		},
	}
	got := sqlite.RowForWrite(map[string]any{
		"id":    1,
		"posts": []map[string]any{{"title": "hi"}},
		"noise": "skip",
	}, schema)
	if got["id"] != 1 {
		t.Fatalf("id=%v", got["id"])
	}
	if _, ok := got["noise"]; ok {
		t.Fatal("noise should be dropped")
	}
	posts, ok := got["posts"].(string)
	if !ok || !strings.Contains(posts, "hi") {
		t.Fatalf("posts=%v", got["posts"])
	}
}

func TestSQLiteNewDestination(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"path": ":memory:"})
	dst, err := sqlite.NewDestination(&models.Connection{
		Type:   models.ConnectionTypeSQLite,
		Config: datatypes.JSON(raw),
	})
	if err != nil {
		t.Fatal(err)
	}
	if dst == nil {
		t.Fatal("expected destination")
	}
}

func TestSQLiteRoundTrip(t *testing.T) {
	path := t.TempDir() + "/test.db"
	raw, _ := json.Marshal(map[string]any{"path": path})
	conn := &models.Connection{
		Type:   models.ConnectionTypeSQLite,
		Config: datatypes.JSON(raw),
	}

	dst, err := sqlite.NewDestination(conn)
	if err != nil {
		t.Fatal(err)
	}
	if err := dst.Open(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer dst.Close()

	schema := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
			{Name: "name", Type: connectors.FieldTypeString},
		},
	}
	if err := dst.Prepare(t.Context(), "people", schema, nil); err != nil {
		t.Fatal(err)
	}
	if err := dst.WriteBatch(t.Context(), "people", []map[string]any{
		{"id": 1, "name": "Ada"},
		{"id": 2, "name": "Bob"},
	}); err != nil {
		t.Fatal(err)
	}

	src, err := sqlite.NewSource(conn)
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Open(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	tables, err := src.ListTables(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, name := range tables {
		if name == "people" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("tables=%v, want people", tables)
	}

	n, err := src.Count(t.Context(), "people", nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("count=%d", n)
	}
}

func TestSQLiteDestinationNestedJSONFilter(t *testing.T) {
	path := t.TempDir() + "/nested.db"
	raw, _ := json.Marshal(map[string]any{"path": path})
	conn := &models.Connection{
		Type:   models.ConnectionTypeSQLite,
		Config: datatypes.JSON(raw),
	}

	dst, err := sqlite.NewDestination(conn)
	if err != nil {
		t.Fatal(err)
	}
	if err := dst.Open(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer dst.Close()

	schema := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
			{Name: "name", Type: connectors.FieldTypeString},
			{Name: "posts", Type: connectors.FieldTypeObjectArray},
		},
	}
	if err := dst.Prepare(t.Context(), "users_test", schema, nil); err != nil {
		t.Fatal(err)
	}
	if err := dst.WriteBatch(t.Context(), "users_test", []map[string]any{
		{"id": 1, "name": "Ada", "posts": []map[string]any{{"body": "hello"}, {"body": "world"}}},
		{"id": 2, "name": "Bob", "posts": []map[string]any{{"body": "other"}}},
		{"id": 3, "name": "Cid", "posts": []map[string]any{}},
	}); err != nil {
		t.Fatal(err)
	}

	reader, ok := dst.(connectors.DestinationReader)
	if !ok {
		t.Fatal("sqlite destination should implement DestinationReader")
	}
	rows, total, err := reader.Query(t.Context(), "users_test", []connectors.Filter{
		{Column: "posts.body", Operator: models.RuleOperatorEq, Value: "hello"},
	}, 50, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("total=%d want 1", total)
	}
	if len(rows) != 1 || fmt.Sprint(rows[0]["name"]) != "Ada" {
		t.Fatalf("rows=%v", rows)
	}
}
