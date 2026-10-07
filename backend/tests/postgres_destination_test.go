package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/postgres"
	"github.com/portico/backend/internal/models"
	"gorm.io/datatypes"
)

func TestPostgresParseConfig(t *testing.T) {
	cfg, err := postgres.ParseConfig([]byte(`{"host":"localhost","database":"app"}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 5432 {
		t.Fatalf("port=%d", cfg.Port)
	}
	if cfg.SSLMode != "disable" || cfg.Schema != "public" {
		t.Fatalf("%+v", cfg)
	}
	if _, err := postgres.ParseConfig([]byte(`{"database":"app"}`)); err == nil {
		t.Fatal("expected host required")
	}
}

func TestPostgresBuildCreateTableSQL(t *testing.T) {
	schema := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
			{Name: "name", Type: connectors.FieldTypeString},
			{Name: "posts", Type: connectors.FieldTypeObjectArray},
			{Name: "meta", Type: connectors.FieldTypeObject},
			{Name: "active", Type: connectors.FieldTypeBool},
			{Name: "score", Type: connectors.FieldTypeFloat64},
		},
	}
	sql, err := postgres.BuildCreateTableSQL("public", "users_copy", schema)
	if err != nil {
		t.Fatal(err)
	}
	wantParts := []string{
		`CREATE TABLE "public"."users_copy"`,
		`"id" BIGINT`,
		`"name" TEXT`,
		`"posts" JSONB`,
		`"meta" JSONB`,
		`"active" BOOLEAN`,
		`"score" DOUBLE PRECISION`,
		`PRIMARY KEY ("id")`,
	}
	for _, part := range wantParts {
		if !strings.Contains(sql, part) {
			t.Fatalf("sql missing %q:\n%s", part, sql)
		}
	}
}

func TestPostgresRowForWrite(t *testing.T) {
	schema := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
			{Name: "posts", Type: connectors.FieldTypeObjectArray},
		},
	}
	got := postgres.RowForWrite(map[string]any{
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

func TestPostgresNewDestination(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"host": "localhost", "port": 5432, "database": "app", "user": "postgres",
	})
	dst, err := postgres.NewDestination(&models.Connection{
		Type:   models.ConnectionTypePostgres,
		Config: datatypes.JSON(raw),
	})
	if err != nil {
		t.Fatal(err)
	}
	if dst == nil {
		t.Fatal("expected destination")
	}
}
